package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Autumn-27/artex/db"
)

// TestInsertAssetsRespectsGlobalAssetIntercept 验证 insert_assets 在真实 PG 上
// 命中全局资产拦截规则时跳过 Upsert，并在 errors 中给出原因。
func TestInsertAssetsRespectsGlobalAssetIntercept(t *testing.T) {
	d := testDB(t)
	defer d.Close()

	task, err := d.CreateTask("agent 资产拦截测试", "目标", nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer d.DeleteTask(task.ID)

	rule, err := d.CreateAssetInterceptRule("exact_domain", "agent-blocked.example", "test", true)
	if err != nil {
		t.Fatal(err)
	}
	defer d.DeleteAssetInterceptRule(rule.ID)

	ts := NewToolSet(nil, "worker")
	ts.SetAssetStore(d.Assets(), d.Companies())
	ts.SetTaskID(task.ID)

	out := callInsertAssets(t, ts, map[string]any{
		"assets": []any{
			map[string]any{"type": "root_domain", "domain": "agent-blocked.example"},
		},
	})
	if errs, _ := out["errors"].([]any); len(errs) == 0 {
		t.Fatalf("被拦截资产应返回 errors，实际: %#v", out)
	}
	var cnt int
	if err := d.QueryRow(`SELECT count(*) FROM assets WHERE domain='agent-blocked.example'`).Scan(&cnt); err != nil {
		t.Fatal(err)
	}
	if cnt != 0 {
		t.Fatalf("被拦截资产不应入库，实际 %d 条", cnt)
	}
}

// TestAddIntentRespectsGlobalAssetIntercept 验证 add_intent 在锚定资产命中
// 全局拦截规则时拒绝下发意图。
func TestAddIntentRespectsGlobalAssetIntercept(t *testing.T) {
	d := testDB(t)
	defer d.Close()

	task, err := d.CreateTask("agent 意图拦截测试", "目标", nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer d.DeleteTask(task.ID)

	assetID, err := d.Assets().UpsertRootDomain(db.UpsertRootDomainReq{
		Domain: "agent-intent-blocked.example",
		TaskID: task.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	rule, err := d.CreateAssetInterceptRule("exact_domain", "agent-intent-blocked.example", "test", true)
	if err != nil {
		t.Fatal(err)
	}
	defer d.DeleteAssetInterceptRule(rule.ID)

	ts := NewToolSet(d.Exploration(task.ExplorationID), "planner")
	ts.SetAssetStore(d.Assets(), d.Companies())
	ts.SetTaskID(task.ID)
	raw, _ := json.Marshal(map[string]any{
		"summary":   "测试被拦截资产",
		"asset_ids": []int64{assetID},
	})
	tool := ts.addIntent()
	res, err := tool.Call(context.Background(), raw, nil)
	if err != nil {
		t.Fatalf("add_intent 调用异常: %v", err)
	}
	if !strings.Contains(res.Flatten(), "未通过测试范围校验") {
		t.Fatalf("锚定被拦截资产时 add_intent 应拒绝，实际返回: %s", res.Flatten())
	}
}

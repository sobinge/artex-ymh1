package db

import "testing"

// TestAssetInterceptEnforcement 验证全局规则、任务级 block/allow 与 AssetStore
// 判定链在真实 PostgreSQL 上的一致性。没有 DSN 时跳过。
func TestAssetInterceptEnforcement(t *testing.T) {
	d := notifyTestDB(t)

	task, err := d.CreateTask("资产拦截集成测试", "目标", nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.DeleteTask(task.ID) })

	assetID, err := d.Assets().UpsertRootDomain(UpsertRootDomainReq{
		Domain: "intercept-global.example",
		TaskID: task.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	global, err := d.CreateAssetInterceptRule("exact_domain", "intercept-global.example", "test global", true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.DeleteAssetInterceptRule(global.ID) })

	hits, err := d.Assets().CheckAssetsIntercept(task.ID, []int64{assetID})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("全局 exact_domain 规则应命中任务资产")
	}

	// 任务级 allow 不应覆盖全局 block（先拦截后允许）。
	if _, err := d.Assets().CreateTaskInterceptRule(task.ID, "allow", "exact_domain", "intercept-global.example", "allow", true); err != nil {
		t.Fatal(err)
	}
	hits, err = d.Assets().CheckAssetsIntercept(task.ID, []int64{assetID})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("任务级 allow 不应绕过全局 block")
	}

	// 构造任务级 block + allow 白名单场景。
	blockRule, err := d.Assets().CreateTaskInterceptRule(task.ID, "block", "fuzzy_domain", ".blocked.example", "", true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = d.Assets().DeleteTaskInterceptRule(task.ID, blockRule.ID) })
	if _, err := d.Assets().CreateTaskInterceptRule(task.ID, "allow", "fuzzy_domain", ".allowed.example", "", true); err != nil {
		t.Fatal(err)
	}

	taskBlock := []AssetInterceptRule{blockRule}
	taskAllow := []AssetInterceptRule{{Kind: "fuzzy_domain", Pattern: ".allowed.example", Action: "allow", Enabled: true}}
	if got := EvaluateAssetGate(taskBlock, taskAllow, []string{"x.allowed.example"}, nil, nil); !got.Allowed {
		t.Fatalf("白名单内资产应放行: %+v", got)
	}
	if got := EvaluateAssetGate(taskBlock, taskAllow, []string{"x.other.example"}, nil, nil); got.Allowed {
		t.Fatalf("白名单外资产应被拒绝: %+v", got)
	}
	if got := EvaluateAssetGate(taskBlock, taskAllow, []string{"x.blocked.example"}, nil, nil); got.Allowed {
		t.Fatalf("命中 block 的资产必须拒绝: %+v", got)
	}
}

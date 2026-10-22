package db

import (
	"context"
	"testing"

	"github.com/Autumn-27/artex/notify"
)

// TestFindingMergeEmitsNotificationEvent 验证同入口合并也会登记推送事件。
// ymh 的 finding merge 会在 RecordFindingTx 里提前返回；如果只在新建路径挂
// 通知，后续证据加深会被静默吞掉。
func TestFindingMergeEmitsNotificationEvent(t *testing.T) {
	d := notifyTestDB(t)

	task, err := d.CreateTask("通知合并测试", "目标", nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer d.DeleteTask(task.ID)

	assetID, err := d.Assets().UpsertRootDomain(UpsertRootDomainReq{
		Domain: "notify-merge.example",
		TaskID: task.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	es := d.Exploration(task.ExplorationID)
	in := RecordFindingInput{
		TaskID:        task.ID,
		ExplorationID: task.ExplorationID,
		VulnClass:     "SQL注入",
		Name:          "合并测试漏洞",
		Severity:      SeverityHigh,
		Summary:       "第一次上报",
		Evidence:      "poc1",
		Worker:        "test",
		AssetIDs:      []int64{assetID},
	}
	first, err := es.RecordFinding(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	in.Summary = "第二次上报（应合并）"
	in.Evidence = "poc2"
	second, err := es.RecordFinding(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Merged || second.FindingID != first.FindingID {
		t.Fatalf("expected merged finding %d, got %+v", first.FindingID, second)
	}

	var created, merged int
	if err := d.QueryRow(`SELECT count(*) FROM notification_events WHERE finding_id=$1 AND kind=$2`, first.FindingID, notify.EventFindingCreated).Scan(&created); err != nil {
		t.Fatal(err)
	}
	if err := d.QueryRow(`SELECT count(*) FROM notification_events WHERE finding_id=$1 AND kind=$2`, first.FindingID, notify.EventFindingMerged).Scan(&merged); err != nil {
		t.Fatal(err)
	}
	if created != 1 || merged != 1 {
		t.Fatalf("notification events = created:%d merged:%d, want 1/1", created, merged)
	}
}

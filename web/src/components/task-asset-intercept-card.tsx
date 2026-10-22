"use client";

import * as React from "react";

import { BanIcon, PlusIcon, Trash2Icon } from "lucide-react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { NativeSelect, NativeSelectOption } from "@/components/ui/native-select";
import { Switch } from "@/components/ui/switch";
import { api } from "@/lib/api";
import type { AssetInterceptKind, AssetInterceptRule } from "@/lib/types";

const KIND_OPTIONS: { value: AssetInterceptKind; label: string }[] = [
  { value: "fuzzy_domain", label: "域名（模糊）" },
  { value: "exact_domain", label: "域名（全等）" },
  { value: "fuzzy_ip", label: "IP（模糊）" },
  { value: "exact_ip", label: "IP（全等）" },
  { value: "fuzzy_url", label: "URL（模糊）" },
  { value: "exact_url", label: "URL（全等）" },
  { value: "cidr", label: "CIDR 网段" },
];

const KIND_LABEL: Record<AssetInterceptKind, string> = Object.fromEntries(
  KIND_OPTIONS.map((o) => [o.value, o.label]),
) as Record<AssetInterceptKind, string>;

/**
 * 任务级资产拦截/允许规则。与全局资产拦截不同，这里的规则只对本任务生效：
 * block = 禁止测试；allow = 白名单（存在 allow 规则时，未命中的资产一律不允许测试）。
 *
 * 真正的执行点在 agent 的 add_intent / insert_assets：规则保存后下一轮工具调用即生效，
 * 不需要重启任务。
 */
export function TaskAssetInterceptCard({ taskId }: { taskId: string }) {
  const [rules, setRules] = React.useState<AssetInterceptRule[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [busy, setBusy] = React.useState(false);
  const [action, setAction] = React.useState<"block" | "allow">("block");
  const [kind, setKind] = React.useState<AssetInterceptKind>("fuzzy_domain");
  const [pattern, setPattern] = React.useState("");
  const [note, setNote] = React.useState("");

  const load = React.useCallback(async () => {
    setLoading(true);
    try {
      setRules(await api.taskInterceptRules(taskId));
    } catch (e) {
      toast.error(`读取任务级资产拦截规则失败：${(e as Error).message}`);
    } finally {
      setLoading(false);
    }
  }, [taskId]);

  React.useEffect(() => {
    void load();
  }, [load]);

  async function add() {
    const p = pattern.trim();
    if (!p) return;
    setBusy(true);
    try {
      await api.createTaskInterceptRule(taskId, {
        action,
        kind,
        pattern: p,
        note: note.trim(),
        enabled: true,
      });
      setPattern("");
      setNote("");
      await load();
      toast.success("已添加任务级资产拦截规则");
    } catch (e) {
      toast.error(`添加失败：${(e as Error).message}`);
    } finally {
      setBusy(false);
    }
  }

  async function toggle(rule: AssetInterceptRule, enabled: boolean) {
    setBusy(true);
    try {
      await api.toggleTaskInterceptRule(taskId, rule.id, enabled);
      setRules((prev) => prev.map((r) => (r.id === rule.id ? { ...r, enabled } : r)));
    } catch (e) {
      toast.error(`切换失败：${(e as Error).message}`);
    } finally {
      setBusy(false);
    }
  }

  async function remove(rule: AssetInterceptRule) {
    setBusy(true);
    try {
      await api.deleteTaskInterceptRule(taskId, rule.id);
      setRules((prev) => prev.filter((r) => r.id !== rule.id));
    } catch (e) {
      toast.error(`删除失败：${(e as Error).message}`);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-base">
          <BanIcon className="size-4 text-red-500" /> 任务级资产拦截
          <span className="text-muted-foreground text-xs font-normal">
            （仅本任务生效；block 禁止测试，allow 为白名单，共 {rules.length} 条）
          </span>
        </CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        <div className="flex flex-wrap items-center gap-2">
          <NativeSelect
            size="sm"
            value={action}
            onChange={(e) => setAction(e.target.value as "block" | "allow")}
            disabled={busy}
          >
            <NativeSelectOption value="block">拦截</NativeSelectOption>
            <NativeSelectOption value="allow">允许</NativeSelectOption>
          </NativeSelect>
          <NativeSelect
            size="sm"
            value={kind}
            onChange={(e) => setKind(e.target.value as AssetInterceptKind)}
            disabled={busy}
          >
            {KIND_OPTIONS.map((o) => (
              <NativeSelectOption key={o.value} value={o.value}>
                {o.label}
              </NativeSelectOption>
            ))}
          </NativeSelect>
          <Input
            className="h-7 min-w-48 flex-1 text-sm"
            placeholder="如 .gov.cn / 10.0.0.0/8 / https://example.com"
            value={pattern}
            onChange={(e) => setPattern(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") void add();
            }}
            disabled={busy}
          />
          <Input
            className="h-7 min-w-40 flex-1 text-sm"
            placeholder="备注（可选）"
            value={note}
            onChange={(e) => setNote(e.target.value)}
            disabled={busy}
          />
          <Button size="sm" variant="outline" disabled={busy || !pattern.trim()} onClick={() => void add()}>
            <PlusIcon className="size-3.5" /> 添加
          </Button>
        </div>
        {loading ? (
          <p className="text-muted-foreground text-sm">加载中…</p>
        ) : rules.length === 0 ? (
          <p className="text-muted-foreground text-sm">
            暂无任务级规则。全局资产拦截仍在生效，可在「系统 → 资产拦截」管理。
          </p>
        ) : (
          <div className="flex flex-col gap-1.5">
            {rules.map((r) => (
              <div key={r.id} className="flex items-center gap-2 rounded-md border px-2.5 py-1.5 text-sm">
                <Badge variant={r.action === "allow" ? "secondary" : "destructive"} className="shrink-0">
                  {r.action === "allow" ? "允许" : "拦截"}
                </Badge>
                <span className="bg-muted text-muted-foreground shrink-0 rounded px-1.5 py-0.5 text-xs">
                  {KIND_LABEL[r.kind] ?? r.kind}
                </span>
                <span className="min-w-0 flex-1 truncate font-mono text-xs" title={r.pattern}>
                  {r.pattern}
                </span>
                {r.note ? (
                  <span className="text-muted-foreground hidden max-w-48 truncate text-xs sm:inline" title={r.note}>
                    {r.note}
                  </span>
                ) : null}
                <Switch
                  checked={r.enabled}
                  disabled={busy}
                  onCheckedChange={(v) => void toggle(r, v)}
                  aria-label="启用规则"
                />
                <Button
                  size="sm"
                  variant="ghost"
                  className="h-6 shrink-0 px-1.5"
                  disabled={busy}
                  onClick={() => void remove(r)}
                >
                  <Trash2Icon className="size-3.5 text-red-500" />
                </Button>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

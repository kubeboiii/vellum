"use client";

import { IconChevronLeft } from "@tabler/icons-react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";

import { Button } from "@/components/Button";
import { Nav } from "@/components/Nav";
import { SeverityBadge } from "@/components/SeverityBadge";
import { StatePill } from "@/components/StatePill";
import { useToast } from "@/components/Toast";
import { getIncident, postRCA } from "@/lib/api";
import { APIError, type FieldError, type RootCauseCategory, type WorkItem } from "@/lib/types";

const CATEGORIES: RootCauseCategory[] = [
  "CODE_DEFECT",
  "INFRASTRUCTURE",
  "CONFIG_CHANGE",
  "EXTERNAL_DEPENDENCY",
  "CAPACITY",
  "HUMAN_ERROR",
  "OTHER",
];

const MIN_TEXT_LENGTH = 20;

function clientValidate(form: {
  fix: string;
  prevention: string;
  category: string;
  submitter: string;
}): FieldError[] {
  const errs: FieldError[] = [];
  if (!form.category) errs.push({ field: "root_cause_category", error: "Pick a category" });
  if (form.fix.trim().length < MIN_TEXT_LENGTH)
    errs.push({ field: "fix_applied", error: `Min ${MIN_TEXT_LENGTH} characters` });
  if (form.prevention.trim().length < MIN_TEXT_LENGTH)
    errs.push({ field: "prevention_steps", error: `Min ${MIN_TEXT_LENGTH} characters` });
  if (!form.submitter.trim()) errs.push({ field: "submitted_by", error: "Required" });
  return errs;
}

function fieldErrorFor(field: string, errs: FieldError[]) {
  return errs.find((e) => e.field === field)?.error;
}

export default function RCAFormPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const id = params.id;

  const [wi, setWI] = useState<WorkItem | null>(null);

  const [category, setCategory] = useState<RootCauseCategory | "">("");
  const [fix, setFix] = useState("");
  const [prevention, setPrevention] = useState("");
  const [submitter, setSubmitter] = useState("sre@example.com");
  const [errors, setErrors] = useState<FieldError[]>([]);
  const [topError, setTopError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const toast = useToast();

  useEffect(() => {
    (async () => {
      try {
        const data = await getIncident(id);
        setWI(data.work_item);
      } catch (e) {
        setTopError((e as Error).message);
      }
    })();
  }, [id]);

  const liveErrors = clientValidate({ fix, prevention, category, submitter });
  const canSubmit = liveErrors.length === 0 && !busy;

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    const local = clientValidate({ fix, prevention, category, submitter });
    if (local.length > 0) {
      setErrors(local);
      setTopError(null);
      return;
    }
    setErrors([]);
    setTopError(null);
    setBusy(true);
    try {
      const res = await postRCA(id, {
        root_cause_category: category,
        fix_applied: fix,
        prevention_steps: prevention,
        submitted_by: submitter,
      });

      toast.push(
        "success",
        `Incident closed · MTTR ${res.work_item.mttr_seconds ?? 0}s`,
      );
      router.push(`/incidents/${id}`);
    } catch (err) {
      if (err instanceof APIError && err.fields && err.fields.length > 0) {
        setErrors(err.fields);
        toast.push("error", `RCA invalid — see fields`);
      } else {
        const msg = (err as Error).message;
        setTopError(msg);
        toast.push("error", msg);
      }
      setBusy(false);
    }
  };

  return (
    <div className="min-h-screen bg-bg-base text-text-primary">
      <Nav title="Submit RCA" />
      <main className="mx-auto max-w-[720px] px-6 py-4 space-y-4">
        <Link
          href={`/incidents/${id}`}
          className="inline-flex items-center gap-1 font-mono text-meta text-text-secondary transition-colors duration-fast hover:text-text-primary"
        >
          <IconChevronLeft size={14} />
          back to incident
        </Link>

        {topError && (
          <div className="rounded-sm border border-sev-p0-border bg-sev-p0-bg/40 px-3 py-2 font-mono text-meta text-red-300">
            {topError}
          </div>
        )}

        {}
        {wi && (
          <section
            className="flex items-center justify-between rounded-md border border-border-subtle bg-bg-surface p-3 transition-[border-color,box-shadow] duration-base ease-out hover:border-border-strong"
            onMouseEnter={(e) => {
              e.currentTarget.style.boxShadow =
                "0 0 24px -10px rgba(190,242,100,0.30)";
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.boxShadow = "";
            }}
          >
            <div className="min-w-0">
              <div className="truncate font-mono text-card font-medium text-text-primary">
                {wi.component_id}
              </div>
              <div className="mt-0.5 font-mono text-meta text-text-tertiary">
                {wi.component_type} · {wi.id.slice(0, 8)} · {wi.signal_count} signals
              </div>
            </div>
            <div className="flex flex-col items-end gap-1.5">
              <SeverityBadge severity={wi.severity} />
              <StatePill status={wi.status} />
            </div>
          </section>
        )}

        <h1 className="relative pb-2 font-sans text-page font-semibold text-text-primary">
          Root Cause Analysis
          <span className="absolute bottom-0 left-0 h-px w-6 bg-accent" aria-hidden />
        </h1>
        <p className="font-sans text-body text-text-secondary">
          On submission, this incident moves to{" "}
          <span className="font-mono text-text-primary">CLOSED</span> and MTTR is computed automatically.
        </p>

        <form onSubmit={onSubmit} className="space-y-4">
          <FormField label="Root cause category" error={fieldErrorFor("root_cause_category", errors)}>
            <select
              value={category}
              onChange={(e) => setCategory(e.target.value as RootCauseCategory)}
              className="h-9 w-full rounded-md border border-border-subtle bg-bg-input px-3 font-sans text-body text-text-primary transition-[border-color,box-shadow] duration-fast focus-visible:border-border-focus focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/45 focus-visible:shadow-[0_0_18px_-8px_rgba(190,242,100,0.55)]"
              disabled={busy}
            >
              <option value="" disabled>— pick one —</option>
              {CATEGORIES.map((c) => (
                <option key={c} value={c}>{c}</option>
              ))}
            </select>
          </FormField>

          <FormField
            label="Fix applied"
            hint={`min ${MIN_TEXT_LENGTH} chars`}
            error={fieldErrorFor("fix_applied", errors)}
          >
            <textarea
              value={fix}
              onChange={(e) => setFix(e.target.value)}
              rows={3}
              className="w-full rounded-md border border-border-subtle bg-bg-input px-3 py-2 font-mono text-data text-text-primary transition-[border-color,box-shadow] duration-fast focus-visible:border-border-focus focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/45 focus-visible:shadow-[0_0_18px_-8px_rgba(190,242,100,0.55)]"
              disabled={busy}
              placeholder="Rebooted the cache cluster and bumped the connection pool from 50 to 200."
            />
            <CharCount value={fix} min={MIN_TEXT_LENGTH} />
          </FormField>

          <FormField
            label="Prevention steps"
            hint={`min ${MIN_TEXT_LENGTH} chars`}
            error={fieldErrorFor("prevention_steps", errors)}
          >
            <textarea
              value={prevention}
              onChange={(e) => setPrevention(e.target.value)}
              rows={3}
              className="w-full rounded-md border border-border-subtle bg-bg-input px-3 py-2 font-mono text-data text-text-primary transition-[border-color,box-shadow] duration-fast focus-visible:border-border-focus focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/45 focus-visible:shadow-[0_0_18px_-8px_rgba(190,242,100,0.55)]"
              disabled={busy}
              placeholder="Add a synthetic monitor for connection-pool saturation; alert on >75% capacity for 60s."
            />
            <CharCount value={prevention} min={MIN_TEXT_LENGTH} />
          </FormField>

          <FormField label="Submitted by" error={fieldErrorFor("submitted_by", errors)}>
            <input
              type="text"
              value={submitter}
              onChange={(e) => setSubmitter(e.target.value)}
              className="h-9 w-full rounded-md border border-border-subtle bg-bg-input px-3 font-mono text-data text-text-primary transition-[border-color,box-shadow] duration-fast focus-visible:border-border-focus focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/45 focus-visible:shadow-[0_0_18px_-8px_rgba(190,242,100,0.55)]"
              disabled={busy}
            />
          </FormField>

          <div className="flex items-center justify-end gap-2 border-t border-border-subtle pt-4">
            <Link href={`/incidents/${id}`}>
              <Button type="button" variant="ghost" disabled={busy}>
                Cancel
              </Button>
            </Link>
            <Button type="submit" variant="primary" disabled={!canSubmit}>
              {busy ? "Submitting…" : "Submit & Close"}
            </Button>
          </div>
        </form>
      </main>
    </div>
  );
}

function FormField({
  label,
  hint,
  error,
  children,
}: {
  label: string;
  hint?: string;
  error?: string;
  children: React.ReactNode;
}) {
  return (
    <div>
      <div className="mb-1 flex items-baseline justify-between">
        <label className="font-sans text-label uppercase tracking-[0.05em] text-text-secondary">
          {label}
        </label>
        {hint && (
          <span className="font-mono text-meta text-text-tertiary">{hint}</span>
        )}
      </div>
      {children}
      {error && (
        <div className="mt-1 font-mono text-meta text-sev-p0">{error}</div>
      )}
    </div>
  );
}

function CharCount({ value, min }: { value: string; min: number }) {
  const len = value.trim().length;
  const ok = len >= min;
  return (
    <div
      className={`mt-1 text-right font-mono text-meta ${ok ? "text-text-tertiary" : "text-warning"}`}
    >
      {len} / {min}
    </div>
  );
}

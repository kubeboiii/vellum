"use client";

import { IconCheck, IconExclamationCircle, IconInfoCircle, IconX } from "@tabler/icons-react";
import { AnimatePresence, motion } from "framer-motion";
import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react";

type Tone = "success" | "error" | "info";

interface Toast {
  id: string;
  tone: Tone;
  message: string;
}

interface ToastCtx {
  push: (tone: Tone, message: string) => void;
}

const Ctx = createContext<ToastCtx>({ push: () => {} });

export function useToast(): ToastCtx {
  return useContext(Ctx);
}

const ICONS: Record<Tone, React.ReactNode> = {
  success: <IconCheck size={14} />,
  error: <IconExclamationCircle size={14} />,
  info: <IconInfoCircle size={14} />,
};

const TONE_CLASSES: Record<Tone, string> = {

  success: "border-emerald-900 bg-emerald-950/40 text-emerald-300",

  error: "border-sev-p0-border bg-sev-p0-bg/60 text-red-300",

  info: "border-accent-border bg-accent-bg text-accent",
};

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);

  const push = useCallback((tone: Tone, message: string) => {
    const id = crypto.randomUUID();
    setToasts((t) => [...t, { id, tone, message }]);
  }, []);

  const dismiss = useCallback((id: string) => {
    setToasts((t) => t.filter((x) => x.id !== id));
  }, []);

  return (
    <Ctx.Provider value={{ push }}>
      {children}
      {}
      <div
        className="pointer-events-none fixed bottom-4 right-4 z-50 flex w-80 flex-col-reverse gap-2"
        aria-live="polite"
      >
        <AnimatePresence initial={false}>
          {toasts.slice(-3).map((t) => (
            <ToastItem key={t.id} toast={t} onDismiss={() => dismiss(t.id)} />
          ))}
        </AnimatePresence>
      </div>
    </Ctx.Provider>
  );
}

function ToastItem({ toast, onDismiss }: { toast: Toast; onDismiss: () => void }) {

  useEffect(() => {
    const t = setTimeout(onDismiss, toast.tone === "error" ? 5000 : 3000);
    return () => clearTimeout(t);
  }, [onDismiss, toast.tone]);

  return (
    <motion.div
      layout
      initial={{ opacity: 0, y: 8, scale: 0.96 }}
      animate={{ opacity: 1, y: 0, scale: 1 }}
      exit={{ opacity: 0, y: 8, scale: 0.96 }}
      transition={{ duration: 0.2, ease: [0.16, 1, 0.3, 1] }}
      className={`pointer-events-auto flex items-start gap-2 rounded-md border px-3 py-2 ${TONE_CLASSES[toast.tone]}`}
      role="status"
    >
      <span className="mt-0.5 flex-shrink-0" aria-hidden>
        {ICONS[toast.tone]}
      </span>
      <span className="flex-1 font-mono text-meta">{toast.message}</span>
      <button
        type="button"
        onClick={onDismiss}
        className="flex-shrink-0 text-text-tertiary transition-colors duration-fast hover:text-text-primary focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent/40"
        aria-label="dismiss"
      >
        <IconX size={14} />
      </button>
    </motion.div>
  );
}

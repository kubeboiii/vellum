export type Persona = "sre" | "commander" | "postmortem";

export const PERSONA_LABELS: Record<Persona, string> = {
  sre: "On-call SRE",
  commander: "Incident commander",
  postmortem: "Post-mortem author",
};

export const PERSONA_DESCRIPTIONS: Record<Persona, string> = {
  sre: "Shows everything currently broken, worst first. Use this when you're fixing things.",
  commander: "Adds counts of how many incidents are at each stage. Use this when you're keeping track of the team.",
  postmortem: "Only shows the ones that have been fixed but still need a write-up. Use this when you're documenting what happened.",
};

export const PERSONA_ROLES: Record<Persona, string> = {
  sre: "For the person fixing it",
  commander: "For the person running the room",
  postmortem: "For the person writing the report",
};

const STORAGE_KEY = "vellum.persona";

export function readPersona(): Persona {
  if (typeof window === "undefined") return "sre";
  const raw = window.localStorage.getItem(STORAGE_KEY);
  if (raw === "sre" || raw === "commander" || raw === "postmortem") {
    return raw;
  }
  return "sre";
}

export function writePersona(p: Persona) {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(STORAGE_KEY, p);
}

import { en } from "./en";
import { es } from "./es";

export type Locale = "es" | "en";

export interface Dictionary {
  readonly appName: string;
  readonly subtitle: string;
  readonly common: Record<string, string>;
  readonly auth: Record<string, string>;
  readonly sidebar: Record<string, string>;
  readonly tunnels: Record<string, string>;
  readonly tunnelModal: Record<string, string>;
  readonly hostModal: Record<string, string>;
  readonly vaultModal: Record<string, string>;
  readonly terminal: Record<string, string>;
  readonly tofu: Record<string, string>;
  readonly sftp: Record<string, string>;
}

const dictionaries: Record<Locale, Dictionary> = { es, en };

let currentLocale: Locale = "es";

export function getLocale(): Locale {
  return currentLocale;
}

export function setLocale(locale: Locale): void {
  currentLocale = locale;
}

export function t(): Dictionary {
  return dictionaries[currentLocale];
}

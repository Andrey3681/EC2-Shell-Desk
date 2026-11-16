import { getLocale, setLocale, t, type Locale } from "../i18n";
import { desktopApi, type UserProfile } from "../services/desktopApi";
import { escapeHtml, extractErrorMessage } from "./dom";

export interface AuthViewOptions {
  hasUsers: boolean;
  onAuthenticated: (user: UserProfile) => void;
  onLocaleChanged: () => void;
}

export function renderAuthView(
  container: HTMLElement,
  opts: AuthViewOptions
): void {
  let mode: "initial" | "login" | "create" = opts.hasUsers ? "login" : "initial";
  let users: UserProfile[] = [];
  let errorMsg = "";

  const loadAndRender = async () => {
    if (opts.hasUsers) {
      try {
        users = await desktopApi.auth.listUsers();
      } catch (err) {
        errorMsg = extractErrorMessage(err);
      }
    }
    draw();
  };

  const draw = () => {
    const copy = t();
    const locale = getLocale();
    const title =
      mode === "initial"
        ? copy.auth.initialTitle
        : mode === "create"
          ? copy.auth.createUserTitle
          : copy.auth.loginTitle;

    const desc =
      mode === "initial" ? copy.auth.initialDesc : copy.auth.loginDesc;

    container.innerHTML = `
      <div class="auth-wrapper">
        <div class="auth-card">
          <div class="auth-header">
            <div>
              <div style="font-size:12px;color:var(--accent-primary);font-weight:700;">${escapeHtml(copy.appName)}</div>
              <h1 class="auth-title">${escapeHtml(title)}</h1>
            </div>
            <select id="auth-locale" class="select" style="width:74px;">
              <option value="es" ${locale === "es" ? "selected" : ""}>ES</option>
              <option value="en" ${locale === "en" ? "selected" : ""}>EN</option>
            </select>
          </div>
          <p class="auth-desc">${escapeHtml(desc)}</p>
          ${errorMsg ? `<div class="error-banner">${escapeHtml(errorMsg)}</div>` : ""}
          <form id="auth-form">
            ${
              mode === "login"
                ? `
              <div class="field">
                <label for="auth-user-select">${escapeHtml(copy.auth.username)}</label>
                <select id="auth-user-select" class="select">
                  ${users
                    .map(
                      (u) =>
                        `<option value="${escapeHtml(u.username)}">${escapeHtml(u.username)}</option>`
                    )
                    .join("")}
                </select>
              </div>`
                : `
              <div class="field">
                <label for="auth-username">${escapeHtml(copy.auth.username)}</label>
                <input id="auth-username" class="input" required minlength="3" autocomplete="off" />
              </div>`
            }
            <div class="field">
              <label for="auth-password">${escapeHtml(copy.auth.password)}</label>
              <input id="auth-password" class="input" type="password" required minlength="8" />
            </div>
            ${
              mode !== "login"
                ? `
              <div class="field">
                <label for="auth-confirm">${escapeHtml(copy.auth.confirmPassword)}</label>
                <input id="auth-confirm" class="input" type="password" required minlength="8" />
              </div>`
                : ""
            }
            <div style="display:flex;gap:8px;margin-top:14px;">
              <button type="submit" class="btn btn-primary" style="flex:1;">
                ${escapeHtml(mode === "initial" ? copy.auth.createInitialUser : mode === "create" ? copy.common.create : copy.auth.signIn)}
              </button>
              ${
                opts.hasUsers
                  ? `<button type="button" id="toggle-auth-mode" class="btn">
                      ${escapeHtml(mode === "login" ? copy.auth.createNewUser : copy.auth.backToLogin)}
                     </button>`
                  : ""
              }
            </div>
          </form>
        </div>
      </div>
    `;

    const localeSel = container.querySelector<HTMLSelectElement>("#auth-locale");
    localeSel?.addEventListener("change", () => {
      setLocale(localeSel.value as Locale);
      opts.onLocaleChanged();
      draw();
    });

    const toggleBtn = container.querySelector<HTMLButtonElement>("#toggle-auth-mode");
    toggleBtn?.addEventListener("click", () => {
      errorMsg = "";
      mode = mode === "login" ? "create" : "login";
      draw();
    });

    const form = container.querySelector<HTMLFormElement>("#auth-form");
    form?.addEventListener("submit", async (e) => {
      e.preventDefault();
      errorMsg = "";
      const password =
        container.querySelector<HTMLInputElement>("#auth-password")?.value ?? "";

      try {
        if (mode === "login") {
          const username =
            container.querySelector<HTMLSelectElement>("#auth-user-select")?.value ?? "";
          const user = await desktopApi.auth.login(username, password);
          opts.onAuthenticated(user);
          return;
        }

        const username =
          container.querySelector<HTMLInputElement>("#auth-username")?.value.trim() ?? "";
        const confirm =
          container.querySelector<HTMLInputElement>("#auth-confirm")?.value ?? "";
        if (password !== confirm) {
          errorMsg = t().auth.passwordMismatch;
          draw();
          return;
        }

        if (mode === "initial") {
          const user = await desktopApi.auth.bootstrapInitialUser(username, password);
          opts.onAuthenticated(user);
        } else {
          await desktopApi.auth.createUser(username, password);
          const user = await desktopApi.auth.login(username, password);
          opts.onAuthenticated(user);
        }
      } catch (err) {
        errorMsg = extractErrorMessage(err);
        draw();
      }
    });
  };

  void loadAndRender();
}

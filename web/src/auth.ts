import { UserManager, WebStorageStateStore, type User } from "oidc-client-ts";

type RuntimeConfig = {
  authRequired: boolean;
  oidcIssuer: string;
  oidcClientId: string;
};

export type AuthState = {
  ready: boolean;
  required: boolean;
  user: User | null;
  error: string;
};

const API = import.meta.env.VITE_API_URL ?? "/api/v1";
let manager: UserManager | null = null;

export async function initialiseAuth(): Promise<AuthState> {
  try {
    const response = await fetch(`${API}/public/config`);
    if (!response.ok)
      throw new Error("Unable to load runtime authentication settings");
    const config = (await response.json()) as RuntimeConfig;
    if (!config.authRequired) {
      sessionStorage.removeItem("suppervisor-token");
      return { ready: true, required: false, user: null, error: "" };
    }
    if (!config.oidcIssuer || !config.oidcClientId) {
      throw new Error("Production authentication is not configured");
    }
    manager = new UserManager({
      authority: config.oidcIssuer,
      client_id: config.oidcClientId,
      redirect_uri: window.location.origin + window.location.pathname,
      post_logout_redirect_uri: window.location.origin,
      response_type: "code",
      scope: "openid profile",
      userStore: new WebStorageStateStore({ store: window.sessionStorage }),
      monitorSession: false,
      automaticSilentRenew: false,
    });
    if (new URLSearchParams(window.location.search).has("code")) {
      await manager.signinRedirectCallback();
      window.history.replaceState({}, document.title, window.location.pathname);
    }
    const user = await manager.getUser();
    if (user && !user.expired && user.id_token) {
      sessionStorage.setItem("suppervisor-token", user.id_token);
      return { ready: true, required: true, user, error: "" };
    }
    sessionStorage.removeItem("suppervisor-token");
    return { ready: true, required: true, user: null, error: "" };
  } catch (error) {
    sessionStorage.removeItem("suppervisor-token");
    const message =
      error instanceof Error ? error.message : "Authentication failed";
    if (import.meta.env.DEV) {
      return {
        ready: true,
        required: false,
        user: null,
        error: message,
      };
    }
    return {
      ready: true,
      required: true,
      user: null,
      error: message,
    };
  }
}

export async function login(): Promise<void> {
  if (!manager) throw new Error("OIDC is not ready");
  await manager.signinRedirect();
}

export async function logout(): Promise<void> {
  sessionStorage.removeItem("suppervisor-token");
  if (manager) await manager.signoutRedirect();
}

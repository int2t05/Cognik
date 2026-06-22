import { apiFetch } from './client';

interface LoginRequest { username: string; password: string; }
interface LoginResponse { access_token: string; refresh_token: string; user: never; roles: string[]; permissions: string[]; menus: never[]; }
interface RefreshResponse { access_token: string; refresh_token: string; }

export function login(data: LoginRequest) { return apiFetch<LoginResponse>('/api/v1/auth/login', { method: 'POST', body: JSON.stringify(data) }); }
export function refreshToken(refresh_token: string) { return apiFetch<RefreshResponse>('/api/v1/auth/refresh', { method: 'POST', body: JSON.stringify({ refresh_token }) }); }
export function changePassword(old_password: string, new_password: string) { return apiFetch<null>('/api/v1/auth/me/change-password', { method: 'POST', body: JSON.stringify({ old_password, new_password }) }); }

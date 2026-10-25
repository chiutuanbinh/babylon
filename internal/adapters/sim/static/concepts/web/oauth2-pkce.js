import { createFlowRecorder, describeFlow } from './flow-steps.js';

// OAuth2 authorization code flow with PKCE (RFC 6749 + RFC 7636). All values
// are fixed and illustrative; nothing here is computed or sent anywhere. The
// verifier/challenge pair is the example from RFC 7636 appendix B.
export const actors = ['Browser', 'App', 'Auth server', 'API'];

export const VERIFIER = 'dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk';
export const CHALLENGE = 'E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM';
export const TOKEN = '2YotnFZFEjr1zCsicMWpAA';

const CODE = 'SplxlOBeZQQYbYS6WxSbIA';
const STATE = 'xyz123';
const REDIRECT = 'https://app.example/callback';
const CLIENT = 'demo-app';

export const describe = describeFlow;

// trace takes no input: the flow is fixed.
export function trace() {
  const r = createFlowRecorder(actors);

  r.state('App', 'code_verifier', VERIFIER);
  r.note('App', 'challenge = base64url(SHA-256(code_verifier)). Only the challenge leaves the app now.');

  r.message('Browser', 'Auth server', 'GET /authorize', [
    ['response_type', 'code'],
    ['client_id', CLIENT],
    ['redirect_uri', REDIRECT],
    ['scope', 'profile'],
    ['state', STATE],
    ['code_challenge', CHALLENGE],
    ['code_challenge_method', 'S256'],
  ]);
  r.message('Auth server', 'Browser', '200 login and consent page');
  r.note('Auth server', 'The user logs in and approves here. The App never sees the password.');
  r.message('Auth server', 'Browser', '302 to redirect_uri', [
    ['code', CODE],
    ['state', STATE],
  ]);
  r.message('Browser', 'App', 'GET /callback', [
    ['code', CODE],
    ['state', STATE],
  ]);
  r.note('App', 'state matches the value sent with /authorize: guards against CSRF.');

  r.message('App', 'Auth server', 'POST /token', [
    ['grant_type', 'authorization_code'],
    ['code', CODE],
    ['redirect_uri', REDIRECT],
    ['client_id', CLIENT],
    ['code_verifier', VERIFIER],
  ]);
  r.note('Auth server', 'SHA-256(code_verifier) matches the stored code_challenge. A stolen code is useless without the verifier.');
  r.message('Auth server', 'App', '200 token response', [
    ['access_token', TOKEN],
    ['token_type', 'Bearer'],
    ['expires_in', '3600'],
  ]);
  r.state('App', 'access_token', TOKEN);

  r.message('App', 'API', 'GET /resource', [['Authorization', `Bearer ${TOKEN}`]]);
  r.message('API', 'App', '200 OK', [['body', '{"name":"Ada"}']]);

  return r.steps;
}

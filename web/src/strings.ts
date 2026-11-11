// Every string the UI shows, in plain en_US (ground rule 9). Translations
// come later by adding another object with the same keys.
export const strings = {
  appName: 'IceDash',
  loading: 'Loading…',
  version: (v: string) => `Version ${v}`,
  signInFirst: 'Sign in to ZimaOS first, then open IceDash again.',
  sessionEnded: 'Your ZimaOS session has ended. Sign in again to continue.',
  cantReach: 'Can’t reach IceDash on this server right now.',
  openZimaOS: 'Open the ZimaOS dashboard',
  adminOnly: 'Only an admin can do this.',
} as const

export type Strings = typeof strings

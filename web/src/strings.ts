// Every string the UI shows, in plain en_US (ground rule 9). Translations
// come later by adding another object with the same keys.
export const strings = {
  appName: 'IceDash',
  loading: 'Loading…',
  version: (v: string) => `Version ${v}`,

  // sign-in
  signInTitle: 'Sign in to ZimaOS first',
  signInBody: 'IceDash uses your ZimaOS account. Sign in on the ZimaOS dashboard, then open IceDash again.',
  sessionEndedTitle: 'Your session has ended',
  sessionEndedBody: 'Sign in to ZimaOS again to keep using IceDash.',
  goToZimaOS: 'Go to ZimaOS',
  cantReach: 'Can’t reach IceDash on this server right now. Retrying…',

  // top bar and the IceDash menu
  desktop: 'Desktop',
  menuLabel: 'IceDash menu',
  menuAbout: 'About IceDash',
  menuCheck: 'Check for updates',
  menuZimaOS: 'Open the ZimaOS dashboard',
  menuSignOut: 'Sign out',
  cpu: 'CPU',
  ram: 'RAM',
  updatesBadge: (n: number) => (n === 1 ? '1 update' : `${n} updates`),
  notificationsButton: (unread: number) => (unread ? `Notifications, ${unread} unread` : 'Notifications'),
  accountButton: (name: string) => `Account: ${name}`,
  togglePanel: 'Show or hide the widgets',

  // launchers
  files: 'Files',
  iceStore: 'IceStore',
  appStore: 'App Store',
  terminal: 'Terminal',
  settings: 'Settings',
  trash: 'Trash',

  // app status
  running: 'Running',
  starting: 'Starting',
  stopped: 'Stopped',
  appStatus: (name: string, status: string) => `${name}, ${status.toLowerCase()}`,
  noApps: 'No apps installed yet.',

  // widgets
  system: 'System',
  memory: 'Memory',
  storage: 'Storage /DATA',
  cores: (n: number) => (n === 1 ? '1 core' : `${n} cores`),
  of: (used: string, total: string) => `${used} of ${total}`,
  upFor: (d: string) => `up ${d}`,
  network: 'Network',
  networkChart: 'Network speed over the last minute',
  apps: 'Apps',
  browseIceStore: 'Find more apps in IceStore',
  open: 'Open →',

  // search
  searchPlaceholder: 'Search apps',
  searchLabel: 'Search apps and launchers',
  searchShortcut: 'Ctrl Space',
  searchNothing: 'Nothing found.',

  // windows
  close: 'Close',
  aboutTitle: 'About IceDash',
  aboutBody: 'A desktop-style dashboard for ZimaOS, in the IceStore design. The stock ZimaOS dashboard stays where it is.',
  installedVersion: 'Installed',
  latestVersion: 'Latest',
  lastChecked: (when: string) => `Last checked ${when}`,
  neverChecked: 'Not checked yet',
  upToDate: 'IceDash is up to date.',
  newVersion: (v: string) => `IceDash ${v} is ready to install.`,
  checkNow: 'Check now',
  checking: 'Checking…',
  updateNow: 'Update now',
  installing: 'Installing…',
  releaseNotes: 'What’s new',
  autoCheck: 'Check for updates automatically',
  autoCheckHint: 'IceDash looks for a new version every 6 hours. It never installs one without you.',
  adminOnly: 'Only an admin can update IceDash.',
  updateFailed: 'The update didn’t install. Nothing on your server changed.',
  restarting: 'IceDash is restarting with the new version…',
  stepState: { running: 'In progress', done: 'Done', failed: 'Failed' } as Record<string, string>,

  // notifications
  notifications: 'Notifications',
  markAllRead: 'Mark all as read',
  clearAll: 'Clear all',
  noNotifications: 'You’re all caught up.',
  notificationTitle: {
    update_available: 'Update available',
    updated: 'IceDash updated',
    update_failed: 'Update failed, previous version restored',
    rollback_failed: 'Rollback failed',
  } as Record<string, string>,
  notificationBody: {
    update_available: (v: string) => `IceDash ${v} is ready to install.`,
    updated: (v: string) => `IceDash is now running ${v}.`,
    update_failed: (v: string) => `The new version didn’t start, so IceDash put ${v} back. Nothing else changed.`,
  } as Record<string, (v: string) => string>,

  // time
  justNow: 'just now',
  minutesAgo: (n: number) => (n === 1 ? '1 minute ago' : `${n} minutes ago`),
  hoursAgo: (n: number) => (n === 1 ? '1 hour ago' : `${n} hours ago`),
  daysAgo: (n: number) => (n === 1 ? 'yesterday' : `${n} days ago`),
  days: (n: number) => (n === 1 ? '1 day' : `${n} days`),
  hours: (n: number) => (n === 1 ? '1 hour' : `${n} hours`),
  minutes: (n: number) => (n === 1 ? '1 minute' : `${n} minutes`),
} as const

export type Strings = typeof strings

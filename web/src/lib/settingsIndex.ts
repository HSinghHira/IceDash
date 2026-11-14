// Every setting search can find (PLAN.md 5b.3), with the Settings page it is
// on. Names come from strings, so they follow the UI's language.
import type { SettingsPage } from '../store'
import { strings } from '../strings'
import type { IconName } from './icons'

export interface SettingHit {
  id: string
  name: string
  keywords: string
  page: SettingsPage
  pageName: string
  icon: IconName
}

export function settingsIndex(opts: { zimaos: boolean }): SettingHit[] {
  const pageName: Record<SettingsPage, string> = {
    general: strings.settingsGeneral,
    appearance: strings.settingsAppearance,
    widgets: strings.settingsWidgets,
    mods: strings.settingsMods,
    dock: strings.settingsDock,
    notifications: strings.settingsNotifications,
    users: strings.settingsUsers,
    about: strings.settingsAbout,
  }
  const icon: Record<SettingsPage, IconName> = {
    general: 'sun',
    appearance: 'palette',
    widgets: 'cards',
    mods: 'eye',
    dock: 'dock',
    notifications: 'bell',
    users: 'users',
    about: 'info',
  }
  const rows: [string, string, SettingsPage, string?][] = [
    ['p-general', strings.settingsGeneral, 'general'],
    ['auto-check', strings.autoCheck, 'general', strings.updatesHeading],
    ['auto-install', strings.autoInstall, 'general', strings.updatesHeading],
    ['server', strings.serverHeading, 'general', `${strings.serverName} ${strings.serverZone}`],
    ['p-appearance', strings.settingsAppearance, 'appearance'],
    ['theme', strings.themeHeading, 'appearance', `${strings.iceName} ${strings.frostName}`],
    ['wallpaper', strings.wallpaperHeading, 'appearance', `${strings.wallUpload} ${strings.wallColor} ${strings.wallDim} ${strings.wallBlur}`],
    ['language', strings.languageHeading, 'appearance', 'English Deutsch Français Español 中文'],
    ['clock', strings.clockHeading, 'appearance', `${strings.clock12} ${strings.clock24}`],
    ['panel', strings.panelShow, 'appearance', strings.panelHeading],
    ['motion', strings.motionReduce, 'appearance', strings.motionHeading],
    ['p-widgets', strings.settingsWidgets, 'widgets', `${strings.addCollection} ${strings.tabCards(0)}`],
    ['p-mods', strings.settingsMods, 'mods', `${strings.tabThemes(0)} ${strings.addCollection}`],
    ['p-dock', strings.settingsDock, 'dock', `${strings.dockLaunchers} ${strings.dockPinned}`],
    ['p-notifications', strings.settingsNotifications, 'notifications'],
    ['quiet', strings.notifyQuiet, 'notifications'],
    ['notify-apps', strings.notifyApps, 'notifications'],
    ['notify-updates', strings.notifyUpdates, 'notifications'],
    ['notify-stopped', strings.notifyStopped, 'notifications'],
    ['p-users', strings.settingsUsers, 'users', `${strings.roleAdmin} ${strings.roleMember}`],
    ['avatar', strings.avatarHeading, 'users', `${strings.avatarDrawn} ${strings.avatarChoose}`],
    ['p-about', strings.settingsAbout, 'about', `${strings.menuCheck} ${strings.updateNow}`],
  ]
  if (opts.zimaos) rows.splice(4, 0, ['main-address', strings.mainAddress, 'general', strings.mainAddressHeading])
  return rows.map(([id, name, page, more]) => ({ id: 's:' + id, name, keywords: `${pageName[page]} ${more ?? ''}`, page, pageName: pageName[page], icon: icon[page] }))
}

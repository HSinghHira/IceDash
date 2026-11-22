# Changelog

## Unreleased

## v1.3.0 (2026-11-22)

- **A new IceStore, laid out like the ZimaOS App Store.** It opens on **Discover**: a big headline beside a banner of featured apps; **Popular** and **Most forked** rows of big screenshot cards; compact three-column lists of **New in IceStore**, **Recently updated** and the biggest categories (icons only, so they load at once); and bundles as spotlight panels with an Install button. The sidebar has the search, Discover, All apps, My apps, Updates, Bundles, and every category with its own icon. Lists have a title and a **Sort** menu: A–Z, Z–A, newest first, recently updated, or the most GitHub stars or forks; IceDash remembers your choice. Cards show stars and forks, and say when an app is installed or has an update. The window is wider too.
- **New app pages.** Like the ZimaOS App Store's: a header card with Install, Open or Update (and Uninstall), four cards for the category, developer, memory it needs and GitHub stars, the screenshots three across (click one to see it full size, and step with the arrow keys), what's new with its version and date, the introduction beside an Info card (source code, docs, CPUs, disk use, date added, Get support), the bundles it belongs to, and more apps from its category.
- **Tidier lists.** All apps, the categories, My apps and Updates show apps as compact rows in three columns: icon, name and two lines, with the stars, forks or date the list is sorted by.
- **Uninstall from IceStore.** An installed app's page has an **Uninstall** button next to Open, with the same question about its data as the right-click menu.
- **Update all.** The Updates list has an **Update all** button that updates every waiting app, one after the other.
- **Only apps for your server.** Apps that don't list your server's CPU (for example amd64-only apps on an ARM board) are hidden, with a count and a **Show apps for other devices** box to list them anyway.

## v1.2.4 (2026-11-20)

- **A live demo.** Try IceDash in your browser at [icedash.hsinghhira.me/demo/](https://icedash.hsinghhira.me/demo/), with no server: a sample desktop with IceStore apps like WatchYourLAN, Ente and AppFlowy, installs from the real IceStore catalog, Files, a small Terminal, Settings, and the official themes and weather card. Nothing is installed anywhere; apps in the demo show what they are instead of running.
- The website opens the demo from its main button, and is a little quieter: no "best viewed on desktop" screen on phones, no drifting background lights, and the tour's tabs wrap on small screens.

## v1.2.3 (2026-11-20)

- **A quieter look.** Small labels are no longer in spaced-out capitals: the cards on the panel (System, Network, Apps), the search groups, the right-click menu, setup, Credentials, First launch, IceStore, Settings and About use ordinary sentence case. Buttons no longer glow, windows and dialogs lose the blue wash in their corner, and apps without an icon get a flat tile. Themes that set `--glow` still work.

## v1.2.2 (2026-11-20)

- **The Weather card changes at once.** Picking a weather place now restarts the cards that use your location, so the new place shows without reloading the page. This also works for a Weather card installed before v1.2.1.
- The weather place search has space below it; it was touching the settings under it.

## v1.2.1 (2026-11-20)

v1.2.0 was tagged but never released: its build stopped at a formatting check, so IceDash's updater kept offering v1.1.0 as the latest. v1.2.1 brings everything in v1.2.0 below, plus, after the third round of comments from the ZimaOS forum:

- **Weather location.** **Settings → Appearance → Weather location**: search for your town and pick it from the list (with its state and country, so you get the right Everett). The Weather card then shows that place instead of the city of your time zone. Widgets with the location permission get the place's coordinates too, and hear when it changes (`IceDash.on('location')`).
- **Apps on the host network install from IceDash.** WatchYourLAN's Install button stayed off without saying why: ZimaOS reported its port as in use, and the popup had no port field to change. A clash the popup can't fix now shows as a warning, and Install goes ahead as it does in ZimaOS's own store.
- **Pick network interfaces.** Settings that ask for network interfaces (like WatchYourLAN's) list this server's interfaces with their addresses: click to add or remove one. The interface of the main network is filled in for you.

## v1.2.0 (2026-11-17), not released

After the second round of comments from the ZimaOS forum:

- **Apps from IceDash can be changed in ZimaOS.** Saving an app's settings in ZimaOS failed with "invalid user override: generated override does not reproduce submitted Compose" for apps IceDash installed. ZimaOS can't build changes on the folded text (`>-`) in IceStore files, so IceDash now writes that text as plain blocks. Apps installed before v1.2.0 keep the old file: to change one, uninstall it (keep its data) and install it again from IceStore.
- **Updates work on ZimaOS.** IceDash updated an app by installing it again, which ZimaOS refuses for an app that is already there ("ports in use"). Updates now go through the same call as ZimaOS's own app settings, and keep the folders you moved or added.
- **Shares connected in ZimaOS show in Files.** Shares you connect in ZimaOS's Files (SMB and cloud drives, mounted by rclone under `/media`) show under **Disks**, as do NFS and SSHFS mounts of a share's subfolder.
- **Folders in setup.** The setup popup has a **Folders** section: point the app's folders somewhere else, or add one, like your media library at `/media/MEDIAS` for `/movies`. Folders under `/DATA`, `/media` or `/mnt` can be used.

## v1.1.0 (2026-11-16)

After the first comments from the ZimaOS forum:

- **Other disks in Files.** USB sticks, second drives and shares mounted under `/media` or `/mnt` show under **Disks** in the Files sidebar, with how much is free. Browse, upload, rename, copy and move work on them as on DATA, and between them. They have no Trash, so Delete asks first and deletes for good.
- **Easier window buttons.** The dots are easier to hit and always show their signs. **Settings → Appearance → Window buttons** switches to Classic: larger − □ × buttons on the right of every title bar.
- **°C or °F.** **Settings → Appearance → Temperature.** It starts on °F in US-region browsers. Widgets are told the choice (`IceDash.on('units')`), and the Weather card follows it.
- The README and the site link to `/modules/icedash/index.html`, which opens on every box.

## v0.4.0 (2026-11-13)

- **Apps open in windows** inside IceDash, with a button to open them in a new tab. Apps that refuse to be shown inside another page (X-Frame-Options or CSP frame-ancestors, checked on the box) open in a new tab as before. Ctrl-click and middle-click still open a new tab.
- **Windows:** the three dots close, minimize and maximize. Minimize shrinks the window into its icon in the dock, and the icon brings it back. Double-click the title bar to maximize. Popups have the same dots.
- **Dock and desktop:** installed apps are on the desktop, filling the first column top to bottom; the dock holds search, Files, IceStore, App Store, Terminal, the open windows, Settings and Trash.
- **Notifications** drop down from the bell, with Mark all read and Clear all as icons in their title bar.
- While a window or the notification list is open, the rest of the desktop dims.
- Everything is smaller: app tiles, dock, popups, menus and IceStore cards. The page no longer scrolls; scrollbars inside are thin and dark.
- `make preview` (or `npm run mock` in web/) shows the desktop on a PC with sample data before anything is committed.

## v0.3.1 (2026-11-12)

- Credentials: an address with its own login in one line (Documenso's mail viewer) shows as an address, a user and a password under its name. The welcome popup shows only the app's own login.
- The welcome popup's steps show this server's address instead of `[ZIMAOS-IP-ADDRESS]` or `your-zimaos-ip`.

## v0.3.0 (2026-11-12)

Phase 2, IceStore inside IceDash (mock-ups 2 to 4):

- **IceStore window:** the IceStore launcher opens IceStore inside IceDash: categories, Installed and Updates, search, app cards with Install, Open or Update, and an app page with screenshots, release notes and links. The catalog is IceStore's 84 KB index, refreshed every 6 hours.
- **Set up before install:** the server's address goes into `[ZIMAOS-IP-ADDRESS]`, `[SERVER-IP]`, `CHANGEME_LAN_IP`, `CHANGEME_ORIGIN` and `CHANGEME_RP_ID`; `CHANGEME_PASSWORD` gets a new secret; the account's email fills an admin email. Ports are checked as you type, with a free one offered when taken. Install stays off until every placeholder is replaced, and the popup says when an app gets full server access, the Docker socket or the host network. Installs go through ZimaOS's own installer, so the app is a normal ZimaOS app.
- **Right-click menu** on any app, desktop or dock: Open, Open in new tab, Credentials, Logs, Restart, Stop or Start, Update, Settings, Back up data (a .tar.gz download) and Uninstall (asks first, keeps the data unless told). Members only get Open. A long press opens it on touch screens.
- **Credentials:** an IceStore app's generated login, read live from its container, never stored; hides after 5 minutes. Admin only.
- **First launch:** the first time an app installed from IceDash is opened, a welcome popup shows its address, login and "How to use" steps, until "Don't show this again".
- **App updates and events:** update dots on app icons, "N app updates in IceStore" in the Apps widget, the top-bar badge counts them, and notifications for installed, install failed, updated, update available and stopped apps. Updating keeps the app's settings and ports.

## v0.2.5 (2026-11-12)

- The System widget shows /DATA in the same units as ZimaOS (140 of 238 GB, not 130 of 222). Memory still counts in 1024s.

## v0.2.4 (2026-11-12)

- While an update installs, the page says "Installing the update…" instead of "Can't reach IceDash".
- After a rollback, the page stops waiting and shows the result.

## v0.2.3 (2026-11-12)

- Rollback test build (`[broken-image]`): the service exits at once. Not meant to be installed; the updater puts the previous version back.

## v0.2.2 (2026-11-12)

- "Update available" shows as soon as Check for updates finds it.
- The update's result ("Updated to …", "Update failed …") shows after the page reloads on the new version.

## v0.2.1 (2026-11-12)

- The update card on the ZimaOS dashboard is removed after an update, even though the new process didn't show it.

## v0.2.0 (2026-11-12)

The desktop from mock-up screen 1:

- Top bar: the IceDash menu (About, Check for updates, Open the ZimaOS dashboard, Sign out), the active window's name, CPU, RAM, download speed, the updates badge, the notification bell, the account menu and the clock (which shows or hides the widgets).
- Desktop icons: Files, IceStore, App Store and Terminal, then every installed app with its status dot. Apps open in a new tab at their own address.
- Dock: launchers, running apps, Settings and Trash.
- Widgets: Clock (time zone as ZimaOS picks it), System (CPU, memory, /DATA, device, uptime), Network (speeds and a 60-second chart), Apps (running, starting, stopped).
- Search (Ctrl+Space): apps and launchers, Enter opens the top match.
- Windows for About and Updates, and for Notifications: dragged, focused, closed with Escape. Toasts for new notifications.
- Update now shows each step, then reloads on the new version.
- Below 900 px the widgets move under the icons and the dock scrolls; no sideways scrolling on a phone.
- The ZimaOS session is refreshed once on a 401 before asking to sign in again.
- Inter and Unbounded ship inside the image: the dashboard calls no other site.
- `ICEDASH_MOCK=1 npm run dev` shows the desktop on a PC, with answers shaped like a real box.

## v0.1.0 (2026-11-11)

First test release: the backend, notifications and self-update.

- The backend: a service on loopback behind the ZimaOS gateway at `/v2/icedash`, which checks the ZimaOS sign-in on every request except `/health` and keeps its state in `/DATA/.icedash`.
- `GET /v2/icedash/health` and `GET /v2/icedash/system` (network speed with a 60-second history, uptime, device name).
- The module image: dashboard tile, service unit and boot watchdog.
- The build: `make check` and `make build`, and CI that publishes releases from version tags only.
- Notifications: the last 100 in `/DATA/.icedash/notifications.json`, read and cleared per user (`GET /notifications`, `POST /notifications/read`, `POST /notifications/clear`).
- Self-update: checks GitHub 2 minutes after start and every 6 hours (switch in `/settings`, on by default), raises "Update available" and the ZimaOS dashboard card. **Update now** (admin only) downloads, verifies the checksum, refuses an image that would replace a ZimaOS file, keeps the installed image, and hands the swap to a transient unit. That unit waits for the new version to start and otherwise puts the previous one back; the result becomes an "Updated to", "Update failed" or "Rollback failed" notification.

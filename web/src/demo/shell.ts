// The live demo's Terminal: a stand-in for the WebSocket the Terminal panel
// opens, with a handful of commands that answer from the sample server.
import { DEVICE, SERVER_IP, startApps } from './data'

const enc = new TextEncoder()
const PROMPT = `\x1b[38;2;76;201;240mroot@${DEVICE}\x1b[0m:\x1b[38;2;165;232;255m/DATA\x1b[0m# `

const COMMANDS: Record<string, () => string> = {
  help: () =>
    [
      'This is the demo: a few commands answer from a sample server.',
      'Try: ls, pwd, whoami, uname -a, uptime, df -h, free -h, docker ps, ip addr, clear, exit',
      'On your own server, this is a real root shell in /DATA.',
    ].join('\r\n'),
  ls: () => 'AppData  Backup  Documents  Downloads  Gallery  Media',
  pwd: () => '/DATA',
  whoami: () => 'root',
  'uname -a': () => `Linux ${DEVICE} 6.12.41-zimaos #1 SMP PREEMPT_DYNAMIC x86_64 GNU/Linux`,
  uptime: () => ' 18:42:07 up 13 days,  4:11,  0 users,  load average: 0.41, 0.37, 0.33',
  'df -h': () =>
    [
      'Filesystem      Size  Used Avail Use% Mounted on',
      '/dev/md0        3.6T  1.5T  2.1T  42% /DATA',
      '/dev/sdc1        58G   27G   31G  47% /media/USB_Stick',
    ].join('\r\n'),
  'free -h': () =>
    ['               total        used        free      shared  buff/cache   available', 'Mem:            31Gi       3.7Gi        21Gi       112Mi       6.1Gi        27Gi'].join('\r\n'),
  'docker ps': () =>
    ['CONTAINER ID   IMAGE                         STATUS          NAMES']
      .concat(
        startApps
          .filter((a) => a.status !== 'exited')
          .map((a, i) => `${(0x3f1a9c2b7e00 + i * 0x1d3f).toString(16).slice(0, 12)}   ${`icestore/${a.name}:latest`.padEnd(29)} Up 13 days      ${a.name}`),
      )
      .join('\r\n'),
  'ip addr': () =>
    [
      '1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536', '    inet 127.0.0.1/8 scope host lo',
      '2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500', `    inet ${SERVER_IP}/24 brd 192.168.1.255 scope global eth0`,
    ].join('\r\n'),
}

export class DemoSocket {
  static readonly CONNECTING = 0
  static readonly OPEN = 1
  static readonly CLOSING = 2
  static readonly CLOSED = 3
  readyState = 0
  binaryType = 'arraybuffer'
  onopen: ((e: Event) => void) | null = null
  onmessage: ((e: MessageEvent) => void) | null = null
  onclose: ((e: CloseEvent) => void) | null = null
  onerror: ((e: Event) => void) | null = null
  private line = ''

  constructor(readonly url: string) {
    setTimeout(() => {
      this.readyState = 1
      this.onopen?.(new Event('open'))
      this.out(`Welcome to the IceDash demo terminal. Type \x1b[1mhelp\x1b[0m to see what works.\r\n\r\n${PROMPT}`)
    }, 150)
  }

  private out(s: string) {
    const data = enc.encode(s).buffer
    this.onmessage?.(new MessageEvent('message', { data }))
  }

  send(d: string) {
    if (this.readyState !== 1 || d.startsWith('\x00')) return
    for (const ch of d) {
      if (ch === '\r') {
        const cmd = this.line.trim()
        this.line = ''
        this.run(cmd)
      } else if (ch === '\x7f') {
        if (this.line) {
          this.line = this.line.slice(0, -1)
          this.out('\b \b')
        }
      } else if (ch === '\x03') {
        this.line = ''
        this.out(`^C\r\n${PROMPT}`)
      } else if (ch >= ' ') {
        this.line += ch
        this.out(ch)
      }
    }
  }

  private run(cmd: string) {
    if (cmd === 'exit') {
      this.out('\r\nlogout\r\n')
      return this.close(1000, '')
    }
    if (cmd === 'clear') return this.out(`\x1b[2J\x1b[H${PROMPT}`)
    const fn = COMMANDS[cmd]
    const text = !cmd ? '' : fn ? fn() : `${cmd.split(' ')[0]}: not in the demo. Type help to see what works.`
    this.out(`\r\n${text ? text + '\r\n' : ''}${PROMPT}`)
  }

  close(code = 1000, reason = '') {
    if (this.readyState === 3) return
    this.readyState = 3
    setTimeout(() => this.onclose?.(new CloseEvent('close', { code, reason })), 0)
  }

  addEventListener() {}
  removeEventListener() {}
}

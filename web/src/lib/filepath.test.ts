import { describe, expect, it } from 'vitest'
import { crumbPaths, parentPath, splitPath, within } from './filepath'

describe('Files paths', () => {
  it('splits /DATA and disk paths', () => {
    expect(splitPath('/')).toEqual({ root: '/', rel: '' })
    expect(splitPath('/Documents/a')).toEqual({ root: '/', rel: 'Documents/a' })
    expect(splitPath('USB_Stick:/')).toEqual({ root: 'USB_Stick:/', rel: '' })
    expect(splitPath('USB_Stick:/Music/x')).toEqual({ root: 'USB_Stick:/', rel: 'Music/x' })
    // A /DATA folder whose name holds ":/" stays in /DATA.
    expect(splitPath('/a:/b')).toEqual({ root: '/', rel: 'a:/b' })
  })
  it('finds the parent', () => {
    expect(parentPath('/')).toBeNull()
    expect(parentPath('/Documents')).toBe('/')
    expect(parentPath('/Documents/a')).toBe('/Documents')
    expect(parentPath('fast:/')).toBeNull()
    expect(parentPath('fast:/Movies')).toBe('fast:/')
    expect(parentPath('fast:/Movies/2024')).toBe('fast:/Movies')
  })
  it('makes crumbs', () => {
    expect(crumbPaths('fast:/Movies/2024')).toEqual([
      { name: 'Movies', path: 'fast:/Movies' },
      { name: '2024', path: 'fast:/Movies/2024' },
    ])
    expect(crumbPaths('/Documents')).toEqual([{ name: 'Documents', path: '/Documents' }])
  })
  it('knows what is inside a folder', () => {
    expect(within('fast:/Movies/a', 'fast:/')).toBe(true)
    expect(within('/Documents/a', '/Documents')).toBe(true)
    expect(within('/Documents2', '/Documents')).toBe(false)
    expect(within('fast:/a', '/')).toBe(false)
  })
})

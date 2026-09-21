import { fireEvent, render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const editorState = vi.hoisted(() => {
  const run = vi.fn(() => true)
  const chain = {
    focus: vi.fn(), extendMarkRange: vi.fn(), unsetLink: vi.fn(), setLink: vi.fn(), undo: vi.fn(), redo: vi.fn(),
    toggleBold: vi.fn(), toggleItalic: vi.fn(), toggleHeading: vi.fn(), toggleBulletList: vi.fn(),
    toggleOrderedList: vi.fn(), toggleBlockquote: vi.fn(), toggleCodeBlock: vi.fn(), run,
  }
  Object.values(chain).forEach((method) => {
    if (method !== run && typeof method === 'function') method.mockReturnValue(chain)
  })
  return {
    html: '<p>Initial</p>',
    options: undefined as undefined | { onUpdate: (value: { editor: { getHTML: () => string } }) => void },
    chain,
    editor: {
      getHTML: vi.fn(() => '<p>Initial</p>'),
      commands: { setContent: vi.fn() },
      can: vi.fn(() => ({ undo: () => true, redo: () => false })),
      isActive: vi.fn((name: string) => name === 'bold'),
      getAttributes: vi.fn(() => ({ href: 'https://old.example' })),
      chain: vi.fn(() => chain),
    },
  }
})

vi.mock('@tiptap/react', () => ({
  useEditor: (options: typeof editorState.options) => {
    editorState.options = options
    return editorState.editor
  },
  EditorContent: () => <div data-testid="editor-content" />,
}))

vi.mock('@tiptap/starter-kit', () => ({ default: {} }))
vi.mock('@tiptap/extension-link', () => ({ default: { configure: vi.fn(() => ({})) } }))

import { RichTextDocEditor } from '../src/components/RichTextDocEditor'

describe('RichTextDocEditor', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    editorState.editor.getHTML.mockReturnValue('<p>Initial</p>')
  })

  it('converts plain text safely and synchronizes external value changes', () => {
    const { rerender } = render(<RichTextDocEditor value={'One < two\n\nNext & final'} onChange={vi.fn()} />)
    expect(editorState.editor.commands.setContent).toHaveBeenCalledWith(
      '<p>One &lt; two</p><p>Next &amp; final</p>',
      { emitUpdate: false },
    )

    editorState.editor.commands.setContent.mockClear()
    editorState.editor.getHTML.mockReturnValue('<p>Saved</p>')
    rerender(<RichTextDocEditor value="<p>Saved</p>" onChange={vi.fn()} />)
    expect(editorState.editor.commands.setContent).not.toHaveBeenCalled()
  })

  it('forwards editor updates as HTML', () => {
    const onChange = vi.fn()
    render(<RichTextDocEditor value="Initial" onChange={onChange} />)
    editorState.options?.onUpdate({ editor: { getHTML: () => '<h1>Architecture</h1>' } })
    expect(onChange).toHaveBeenCalledWith('<h1>Architecture</h1>')
  })

  it('runs formatting commands and reflects disabled/active states', () => {
    render(<RichTextDocEditor value="Initial" onChange={vi.fn()} />)
    expect(screen.getByTitle('Bold').className).toBe('active')
    expect((screen.getByTitle('Redo') as HTMLButtonElement).disabled).toBe(true)
    fireEvent.click(screen.getByTitle('Bold'))
    fireEvent.click(screen.getByTitle('Heading 1'))
    fireEvent.click(screen.getByTitle('Bullet list'))
    expect(editorState.chain.toggleBold).toHaveBeenCalled()
    expect(editorState.chain.toggleHeading).toHaveBeenCalledWith({ level: 1 })
    expect(editorState.chain.toggleBulletList).toHaveBeenCalled()
  })

  it('sets and removes links through the URL prompt', () => {
    const prompt = vi.spyOn(window, 'prompt')
    prompt.mockReturnValueOnce(' https://stratum.example/docs ')
    render(<RichTextDocEditor value="Initial" onChange={vi.fn()} />)
    fireEvent.click(screen.getByTitle('Link'))
    expect(editorState.chain.setLink).toHaveBeenCalledWith({ href: 'https://stratum.example/docs' })

    prompt.mockReturnValueOnce('')
    fireEvent.click(screen.getByTitle('Link'))
    expect(editorState.chain.unsetLink).toHaveBeenCalled()
    prompt.mockRestore()
  })
})

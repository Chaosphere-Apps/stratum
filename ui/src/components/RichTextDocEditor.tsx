import { useEffect } from 'react'
import { EditorContent, useEditor, type Editor } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import LinkExtension from '@tiptap/extension-link'
import {
  Bold,
  Code2,
  Heading1,
  Heading2,
  Italic,
  Link as LinkIcon,
  List,
  ListOrdered,
  Quote,
  Redo2,
  Undo2,
} from 'lucide-react'

function documentBodyToEditorContent(body: string) {
  const trimmed = body.trim()
  if (!trimmed) return '<p></p>'
  if (trimmed.startsWith('<')) return body
  return body
    .split(/\n{2,}/)
    .map((paragraph) => `<p>${escapeHtml(paragraph).replace(/\n/g, '<br>')}</p>`)
    .join('')
}

function escapeHtml(value: string) {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#039;')
}

export function RichTextDocEditor({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  const editor = useEditor({
    extensions: [
      StarterKit,
      LinkExtension.configure({
        openOnClick: false,
        autolink: true,
      }),
    ],
    content: documentBodyToEditorContent(value),
    editorProps: {
      attributes: {
        class: 'rich-doc-editor-content',
      },
    },
    onUpdate: ({ editor: updatedEditor }) => {
      onChange(updatedEditor.getHTML())
    },
  })

  useEffect(() => {
    if (!editor) return
    const nextContent = documentBodyToEditorContent(value)
    if (editor.getHTML() === nextContent) return
    editor.commands.setContent(nextContent, { emitUpdate: false })
  }, [editor, value])

  if (!editor) {
    return <div className="rich-doc-editor-shell loading" />
  }

  return (
    <div className="rich-doc-editor-shell">
      <RichTextToolbar editor={editor} />
      <EditorContent editor={editor} />
    </div>
  )
}

function RichTextToolbar({ editor }: { editor: Editor }) {
  function setLink() {
    const previousUrl = editor.getAttributes('link').href as string | undefined
    const url = window.prompt('URL', previousUrl ?? '')
    if (url === null) return
    if (!url.trim()) {
      editor.chain().focus().extendMarkRange('link').unsetLink().run()
      return
    }
    editor.chain().focus().extendMarkRange('link').setLink({ href: url.trim() }).run()
  }

  const buttons = [
    {
      title: 'Undo',
      icon: <Undo2 size={16} />,
      active: false,
      disabled: !editor.can().undo(),
      action: () => editor.chain().focus().undo().run(),
    },
    {
      title: 'Redo',
      icon: <Redo2 size={16} />,
      active: false,
      disabled: !editor.can().redo(),
      action: () => editor.chain().focus().redo().run(),
    },
    {
      title: 'Bold',
      icon: <Bold size={16} />,
      active: editor.isActive('bold'),
      action: () => editor.chain().focus().toggleBold().run(),
    },
    {
      title: 'Italic',
      icon: <Italic size={16} />,
      active: editor.isActive('italic'),
      action: () => editor.chain().focus().toggleItalic().run(),
    },
    {
      title: 'Heading 1',
      icon: <Heading1 size={16} />,
      active: editor.isActive('heading', { level: 1 }),
      action: () => editor.chain().focus().toggleHeading({ level: 1 }).run(),
    },
    {
      title: 'Heading 2',
      icon: <Heading2 size={16} />,
      active: editor.isActive('heading', { level: 2 }),
      action: () => editor.chain().focus().toggleHeading({ level: 2 }).run(),
    },
    {
      title: 'Bullet list',
      icon: <List size={16} />,
      active: editor.isActive('bulletList'),
      action: () => editor.chain().focus().toggleBulletList().run(),
    },
    {
      title: 'Numbered list',
      icon: <ListOrdered size={16} />,
      active: editor.isActive('orderedList'),
      action: () => editor.chain().focus().toggleOrderedList().run(),
    },
    {
      title: 'Quote',
      icon: <Quote size={16} />,
      active: editor.isActive('blockquote'),
      action: () => editor.chain().focus().toggleBlockquote().run(),
    },
    {
      title: 'Code block',
      icon: <Code2 size={16} />,
      active: editor.isActive('codeBlock'),
      action: () => editor.chain().focus().toggleCodeBlock().run(),
    },
    {
      title: 'Link',
      icon: <LinkIcon size={16} />,
      active: editor.isActive('link'),
      action: setLink,
    },
  ]

  return (
    <div className="rich-doc-toolbar">
      {buttons.map((button) => (
        <button
          className={button.active ? 'active' : ''}
          type="button"
          key={button.title}
          title={button.title}
          disabled={button.disabled}
          onClick={button.action}
        >
          {button.icon}
        </button>
      ))}
    </div>
  )
}

'use client';

import {useEffect, useState} from 'react';
import {EditorContent, useEditor} from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import LinkExtension from '@tiptap/extension-link';
import ImageExtension from '@tiptap/extension-image';
import Placeholder from '@tiptap/extension-placeholder';
import {
  BoldIcon,
  Code2Icon,
  CodeIcon,
  Heading2Icon,
  ImageIcon,
  ItalicIcon,
  Link2Icon,
  ListIcon,
  ListOrderedIcon,
  QuoteIcon,
  Redo2Icon,
  Undo2Icon,
} from 'lucide-react';
import {cn} from 'cn';
import {Button} from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {Field, FieldError, FieldLabel} from '@/components/ui/field';
import {Input} from '@/components/ui/input';
import {Toggle} from '@/components/ui/toggle';
import {isSafeUrl} from './tiptap_renderer';

export interface TiptapEditorProps {
  value: string;
  onChange: (html: string) => void;
  mode?: 'article' | 'comment';
  placeholder?: string;
  disabled?: boolean;
  minHeightClass?: string;
  ariaLabel?: string;
  invalid?: boolean;
}

/**
 * 基于 Tiptap 3.x 与官方 shadcn/ui Radix 工具栏的富文本编辑器（设计 010 第 7 节）
 * - article 模式：段落、H2/H3 标题、无序/有序列表、引用、链接、行内/块代码、图片、撤销/重做
 * - comment 精简模式：段落、加粗/斜体、链接、行内代码
 */
export function TiptapEditor({
  value,
  onChange,
  mode = 'article',
  placeholder = '输入正文内容…',
  disabled = false,
  minHeightClass,
  ariaLabel = '富文本编辑区',
  invalid = false,
}: TiptapEditorProps) {
  const [linkDialogOpen, setLinkDialogOpen] = useState(false);
  const [linkUrl, setLinkUrl] = useState('');
  const [linkError, setLinkError] = useState('');

  const [imageDialogOpen, setImageDialogOpen] = useState(false);
  const [imageUrl, setImageUrl] = useState('');
  const [imageAlt, setImageAlt] = useState('');
  const [imageError, setImageError] = useState('');

  const editor = useEditor({
    immediatelyRender: false,
    editable: !disabled,
    extensions: [
      StarterKit.configure({
        heading: mode === 'article' ? {levels: [2, 3]} : false,
        bulletList: mode === 'article' ? {} : false,
        orderedList: mode === 'article' ? {} : false,
        blockquote: mode === 'article' ? {} : false,
        codeBlock: mode === 'article' ? {} : false,
        horizontalRule: false,
      }),
      LinkExtension.configure({
        openOnClick: false,
        autolink: true,
        HTMLAttributes: {
          rel: 'noopener noreferrer',
          target: '_blank',
        },
      }),
      ...(mode === 'article'
        ? [
            ImageExtension.configure({
              allowBase64: false,
            }),
          ]
        : []),
      Placeholder.configure({
        placeholder,
      }),
    ],
    content: value,
    editorProps: {
      attributes: {
        role: 'textbox',
        'aria-multiline': 'true',
        'aria-label': ariaLabel,
        class: cn(
          'learnis-prose outline-none px-4 py-3 text-base leading-7',
          minHeightClass ?? (mode === 'article' ? 'min-h-56' : 'min-h-24'),
        ),
      },
    },
    onUpdate: ({editor: currentEditor}) => {
      onChange(currentEditor.getHTML());
    },
  });

  useEffect(() => {
    if (!editor) {
      return;
    }
    editor.setEditable(!disabled);
  }, [disabled, editor]);

  useEffect(() => {
    if (!editor) {
      return;
    }
    if (value !== editor.getHTML()) {
      editor.commands.setContent(value || '', {emitUpdate: false});
    }
  }, [editor, value]);

  function openLinkDialog() {
    if (!editor) {
      return;
    }
    const existing = editor.getAttributes('link').href as string | undefined;
    setLinkUrl(existing ?? '');
    setLinkError('');
    setLinkDialogOpen(true);
  }

  function applyLink() {
    if (!editor) {
      return;
    }
    const trimmed = linkUrl.trim();
    if (!trimmed) {
      editor.chain().focus().extendMarkRange('link').unsetLink().run();
      setLinkDialogOpen(false);
      return;
    }
    if (!isSafeUrl(trimmed)) {
      setLinkError('请输入有效的 http://、https:// 或站内 / 开头链接');
      return;
    }
    editor
      .chain()
      .focus()
      .extendMarkRange('link')
      .setLink({href: trimmed})
      .run();
    setLinkDialogOpen(false);
  }

  function applyImage() {
    if (!editor) {
      return;
    }
    const trimmed = imageUrl.trim();
    if (!isSafeUrl(trimmed)) {
      setImageError('请输入有效的 http://、https:// 图片地址');
      return;
    }
    editor
      .chain()
      .focus()
      .setImage({src: trimmed, alt: imageAlt.trim() || '正文配图'})
      .run();
    setImageUrl('');
    setImageAlt('');
    setImageError('');
    setImageDialogOpen(false);
  }

  const currentHeadingLabel = editor?.isActive('heading', {level: 2})
    ? '标题二'
    : editor?.isActive('heading', {level: 3})
      ? '标题三'
      : '正文';

  return (
    <div
      className={cn(
        'overflow-hidden rounded-xl bg-control-surface transition-colors focus-within:ring-2 focus-within:ring-ring focus-within:ring-offset-2 focus-within:ring-offset-background',
        invalid && 'bg-destructive-surface ring-2 ring-ring/40',
        disabled && 'opacity-60',
      )}
    >
      <div
        role="toolbar"
        aria-label="编辑工具栏"
        className="flex flex-wrap items-center gap-1 overflow-x-auto bg-canvas-soft px-2.5 py-2"
      >
        {mode === 'article' ? (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                disabled={disabled || !editor}
              >
                <Heading2Icon data-icon="inline-start" />
                {currentHeadingLabel}
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start">
              <DropdownMenuGroup>
                <DropdownMenuItem
                  onSelect={() => editor?.chain().focus().setParagraph().run()}
                >
                  正文段落
                </DropdownMenuItem>
                <DropdownMenuItem
                  onSelect={() =>
                    editor?.chain().focus().toggleHeading({level: 2}).run()
                  }
                >
                  二级标题 (H2)
                </DropdownMenuItem>
                <DropdownMenuItem
                  onSelect={() =>
                    editor?.chain().focus().toggleHeading({level: 3}).run()
                  }
                >
                  三级标题 (H3)
                </DropdownMenuItem>
              </DropdownMenuGroup>
            </DropdownMenuContent>
          </DropdownMenu>
        ) : null}

        <Toggle
          size="sm"
          aria-label="加粗"
          pressed={Boolean(editor?.isActive('bold'))}
          disabled={disabled || !editor}
          onPressedChange={() => editor?.chain().focus().toggleBold().run()}
        >
          <BoldIcon />
        </Toggle>

        <Toggle
          size="sm"
          aria-label="斜体"
          pressed={Boolean(editor?.isActive('italic'))}
          disabled={disabled || !editor}
          onPressedChange={() => editor?.chain().focus().toggleItalic().run()}
        >
          <ItalicIcon />
        </Toggle>

        <Toggle
          size="sm"
          aria-label="行内代码"
          pressed={Boolean(editor?.isActive('code'))}
          disabled={disabled || !editor}
          onPressedChange={() => editor?.chain().focus().toggleCode().run()}
        >
          <CodeIcon />
        </Toggle>

        <Button
          type="button"
          variant={editor?.isActive('link') ? 'secondary' : 'ghost'}
          size="icon-sm"
          aria-label="插入或编辑链接"
          disabled={disabled || !editor}
          onClick={openLinkDialog}
        >
          <Link2Icon />
        </Button>

        {mode === 'article' ? (
          <>
            <Toggle
              size="sm"
              aria-label="无序列表"
              pressed={Boolean(editor?.isActive('bulletList'))}
              disabled={disabled || !editor}
              onPressedChange={() =>
                editor?.chain().focus().toggleBulletList().run()
              }
            >
              <ListIcon />
            </Toggle>
            <Toggle
              size="sm"
              aria-label="有序列表"
              pressed={Boolean(editor?.isActive('orderedList'))}
              disabled={disabled || !editor}
              onPressedChange={() =>
                editor?.chain().focus().toggleOrderedList().run()
              }
            >
              <ListOrderedIcon />
            </Toggle>
            <Toggle
              size="sm"
              aria-label="引用块"
              pressed={Boolean(editor?.isActive('blockquote'))}
              disabled={disabled || !editor}
              onPressedChange={() =>
                editor?.chain().focus().toggleBlockquote().run()
              }
            >
              <QuoteIcon />
            </Toggle>
            <Toggle
              size="sm"
              aria-label="代码块"
              pressed={Boolean(editor?.isActive('codeBlock'))}
              disabled={disabled || !editor}
              onPressedChange={() =>
                editor?.chain().focus().toggleCodeBlock().run()
              }
            >
              <Code2Icon />
            </Toggle>
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              aria-label="插入图片"
              disabled={disabled || !editor}
              onClick={() => {
                setImageError('');
                setImageDialogOpen(true);
              }}
            >
              <ImageIcon />
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              aria-label="撤销"
              disabled={disabled || !editor?.can().undo()}
              onClick={() => editor?.chain().focus().undo().run()}
            >
              <Undo2Icon />
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              aria-label="重做"
              disabled={disabled || !editor?.can().redo()}
              onClick={() => editor?.chain().focus().redo().run()}
            >
              <Redo2Icon />
            </Button>
          </>
        ) : null}
      </div>

      <EditorContent editor={editor} />

      <Dialog open={linkDialogOpen} onOpenChange={setLinkDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>插入链接</DialogTitle>
            <DialogDescription>
              仅支持 http://、https:// 或站内路径，留空确认可移除当前链接。
            </DialogDescription>
          </DialogHeader>
          <Field data-invalid={Boolean(linkError)}>
            <FieldLabel htmlFor="tiptap-link-url">目标地址</FieldLabel>
            <Input
              id="tiptap-link-url"
              type="url"
              placeholder="https://example.com"
              value={linkUrl}
              aria-invalid={Boolean(linkError)}
              onChange={(e) => {
                setLinkUrl(e.target.value);
                setLinkError('');
              }}
            />
            {linkError ? <FieldError>{linkError}</FieldError> : null}
          </Field>
          <DialogFooter>
            <Button
              type="button"
              variant="secondary"
              onClick={() => setLinkDialogOpen(false)}
            >
              取消
            </Button>
            <Button type="button" onClick={applyLink}>
              应用链接
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={imageDialogOpen} onOpenChange={setImageDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>插入正文图片</DialogTitle>
            <DialogDescription>
              填写可公开或受控访问的 HTTPS 图片地址与替代说明文本。
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-4">
            <Field data-invalid={Boolean(imageError)}>
              <FieldLabel htmlFor="tiptap-image-url">图片地址</FieldLabel>
              <Input
                id="tiptap-image-url"
                type="url"
                placeholder="https://..."
                value={imageUrl}
                aria-invalid={Boolean(imageError)}
                onChange={(e) => {
                  setImageUrl(e.target.value);
                  setImageError('');
                }}
              />
              {imageError ? <FieldError>{imageError}</FieldError> : null}
            </Field>
            <Field>
              <FieldLabel htmlFor="tiptap-image-alt">替代文本 (alt)</FieldLabel>
              <Input
                id="tiptap-image-alt"
                type="text"
                placeholder="例如：架构示意图"
                value={imageAlt}
                onChange={(e) => setImageAlt(e.target.value)}
              />
            </Field>
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="secondary"
              onClick={() => setImageDialogOpen(false)}
            >
              取消
            </Button>
            <Button type="button" onClick={applyImage}>
              插入图片
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

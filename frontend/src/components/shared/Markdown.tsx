interface Props {
  content: string
  className?: string
}

function escapeHtml(text: string): string {
  return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

export default function Markdown({ content, className = '' }: Props) {
  const html = convertToHtml(content)
  return (
    <div
      className={`prose prose-sm max-w-none prose-headings:mb-2 prose-p:my-1 prose-code:bg-gray-100 prose-code:px-1 prose-code:rounded prose-pre:bg-gray-900 prose-pre:text-gray-100 prose-ul:my-1 prose-li:my-0.5 ${className}`}
      dangerouslySetInnerHTML={{ __html: html }}
    />
  )
}

function convertToHtml(text: string): string {
  let html = escapeHtml(text)

  // code blocks (```)
  html = html.replace(/```(\w*)\n([\s\S]*?)```/g, '<pre><code>$2</code></pre>')

  // inline code
  html = html.replace(/`([^`]+)`/g, '<code>$1</code>')

  // headers
  html = html.replace(/^### (.+)$/gm, '<h3>$1</h3>')
  html = html.replace(/^## (.+)$/gm, '<h2>$1</h2>')
  html = html.replace(/^# (.+)$/gm, '<h1>$1</h1>')

  // bold
  html = html.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')

  // italic
  html = html.replace(/\*(.+?)\*/g, '<em>$1</em>')

  // strikethrough
  html = html.replace(/~~(.+?)~~/g, '<del>$1</del>')

  // unordered lists
  html = html.replace(/^[\s]*[-*+] (.+)$/gm, '<li>$1</li>')
  html = html.replace(/(<li>.*<\/li>\n?)+/g, '<ul>$&</ul>')

  // ordered lists
  html = html.replace(/^[\s]*\d+\.\s(.+)$/gm, '<li>$1</li>')
  html = html.replace(/(<li>.*<\/li>\n?)+/g, (match) => {
    if (match.includes('<ul>')) return match
    return '<ol>' + match + '</ol>'
  })

  // horizontal rules
  html = html.replace(/^---$/gm, '<hr />')

  // blockquote
  html = html.replace(/^> (.+)$/gm, '<blockquote>$1</blockquote>')

  // paragraphs (double newlines)
  html = html.replace(/\n\n/g, '</p><p>')

  // line breaks
  html = html.replace(/\n/g, '<br />')

  // wrap in paragraph if not starting with HTML tag
  if (!html.startsWith('<')) {
    html = '<p>' + html + '</p>'
  }

  return html
}

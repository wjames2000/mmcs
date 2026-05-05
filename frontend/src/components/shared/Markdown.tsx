interface Props {
  content: string
  className?: string
  fontSize?: number
}

function escapeHtml(text: string): string {
  return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

export default function Markdown({ content, className = '', fontSize }: Props) {
  const html = convertToHtml(content)
  return (
    <div
      className={`max-w-none prose-headings:mb-2 prose-p:my-1 prose-code:bg-gray-100 prose-code:px-1 prose-code:rounded prose-pre:bg-gray-900 prose-pre:text-gray-100 prose-ul:my-1 prose-li:my-0.5 prose-table:border-collapse prose-table:w-full prose-th:border prose-th:border-gray-300 prose-th:px-3 prose-th:py-1.5 prose-th:bg-gray-50 prose-td:border prose-td:border-gray-300 prose-td:px-3 prose-td:py-1.5 ${className}`}
      style={{ fontSize: fontSize ? `${fontSize}px` : undefined }}
      dangerouslySetInnerHTML={{ __html: html }}
    />
  )
}

function convertToHtml(text: string): string {
  // Preserve code blocks before escaping
  const codeBlocks: string[] = []
  let html = text.replace(/```(\w*)\n([\s\S]*?)```/g, (_match, _lang, code) => {
    codeBlocks.push(code)
    return `\x00CODEBLOCK${codeBlocks.length - 1}\x00`
  })

  // Preserve inline code
  const inlineCodes: string[] = []
  html = html.replace(/`([^`]+)`/g, (_match, code) => {
    inlineCodes.push(code)
    return `\x00INLINECODE${inlineCodes.length - 1}\x00`
  })

  // Escape HTML
  html = escapeHtml(html)

  // Restore inline code
  html = html.replace(/\x00INLINECODE(\d+)\x00/g, (_match, id) => {
    return `<code>${inlineCodes[parseInt(id)]}</code>`
  })

  // Tables (must be before paragraph wrapping)
  html = html.replace(/^\|(.+)\|\n\|[-| :]+\|\n((?:\|.+\|\n?)*)/gm, (_match, headerRow, bodyRows) => {
    const headers = headerRow.split('|').filter((s: string) => s.trim()).map((s: string) => s.trim())
    const rows = bodyRows.trim().split('\n').filter((r: string) => r.trim())
    let table = '<table><thead><tr>'
    for (const h of headers) {
      table += `<th>${h}</th>`
    }
    table += '</tr></thead><tbody>'
    for (const row of rows) {
      const parts = row.split('|')
      const cells = parts.slice(1, -1).map((s: string) => s.trim())
      if (cells.length === 0) continue
      table += '<tr>'
      for (const cell of cells) {
        // Also render simple inline formatting in table cells
        let c = cell
        c = c.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
        c = c.replace(/`([^`]+)`/g, '<code>$1</code>')
        table += `<td>${c}</td>`
      }
      table += '</tr>'
    }
    table += '</tbody></table>'
    return table
  })

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

  // Restore code blocks (after other transformations, before paragraph wrapping)
  html = html.replace(/\x00CODEBLOCK(\d+)\x00/g, (_match, id) => {
    return `<pre><code>${escapeHtml(codeBlocks[parseInt(id)])}</code></pre>`
  })

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

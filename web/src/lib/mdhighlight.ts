// Colours for the editor's Markdown, as HTML laid out exactly like the
// text: every character of the source comes out once, in place, so the
// coloured copy can sit under the textarea. Only colour changes — bold or
// italics would change the width of the letters and pull the copy out of
// line with the caret.

const esc = (s: string) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
const span = (cls: string, s: string) => (s ? `<span class="md-${cls}">${esc(s)}</span>` : '')

// Inline syntax, tried left to right; the first alternative that matches at
// a position wins. Groups are numbered across the alternatives: code 1–2,
// wikilinks 3–6, date anchors 7–10, footnotes 11, links 12–16, bold 17–18,
// italics 19–20, strike 21–22, highlight 23–24.
const inline = new RegExp(
  [
    /(`+)([^`]|[^`][\s\S]*?[^`])\1(?!`)/.source, // `code`
    /(!?\[\[)([^\]|]*)(\|[^\]]*)?(\]\])/.source, // [[link|label]], ![[embed]]
    /(\{\{)([^{}|\n]*)(\|[^{}\n]*)?(\}\})/.source, // {{date | description}}
    /(\[\^[^\]]+\])/.source, // [^1] footnote
    /(!?\[)([^\]\n]*)(\]\()([^)\n]*)(\))/.source, // [text](url), ![alt](src)
    String.raw`(\*\*|__)(?=\S)([^\n]*?\S)\17`, // **bold** (\17: its own opening mark)
    String.raw`(\*|_)(?=\S)([^\n*_]*?\S)\19(?![\p{L}\p{N}])`, // *italic*
    /(~~)(?=\S)([^\n]*?\S)~~/.source, // ~~strike~~
    /(==)(?=\S)([^\n]*?\S)==/.source, // ==highlight==
  ].join('|'),
  'gu',
)

function inlineHTML(s: string): string {
  let out = ''
  let last = 0
  for (const m of s.matchAll(inline)) {
    const at = m.index!
    out += esc(s.slice(last, at))
    const [whole] = m
    if (m[1] !== undefined) {
      out += span('code', whole)
    } else if (m[3] !== undefined) {
      const embed = m[3].startsWith('!')
      const label = m[5] ?? ''
      out += span('mark', m[3]) + span(embed ? 'embed' : 'link', m[4]) + span('mark', label.slice(0, 1)) + span('label', label.slice(1)) + span('mark', m[6])
    } else if (m[7] !== undefined) {
      out += span('mark', m[7]) + span('date', m[8]) + span('note', m[9] ?? '') + span('mark', m[10])
    } else if (m[11] !== undefined) {
      out += span('mark', m[11])
    } else if (m[12] !== undefined) {
      out += span('mark', m[12]) + span(m[12].startsWith('!') ? 'embed' : 'link', m[13]) + span('mark', m[14]) + span('url', m[15]) + span('mark', m[16])
    } else if (m[17] !== undefined) {
      out += span('mark', m[17]) + span('strong', m[18]) + span('mark', m[17])
    } else if (m[19] !== undefined) {
      out += span('mark', m[19]) + span('em', m[20]) + span('mark', m[19])
    } else if (m[21] !== undefined) {
      out += span('mark', '~~') + span('strike', m[22]) + span('mark', '~~')
    } else {
      out += span('mark', '==') + span('highlight', m[24]) + span('mark', '==')
    }
    last = at + whole.length
  }
  return out + esc(s.slice(last))
}

export function highlightMarkdown(text: string): string {
  const lines = text.split('\n')
  const out: string[] = []
  let fence = '' // the ``` or ~~~ that opened a code block, while inside one
  for (const line of lines) {
    if (fence) {
      out.push(span('code', line))
      if (line.trimStart().startsWith(fence)) fence = ''
      continue
    }
    const opening = line.match(/^\s*(```+|~~~+)/)
    if (opening) {
      fence = opening[1]
      out.push(span('code', line))
      continue
    }
    let m: RegExpMatchArray | null
    if ((m = line.match(/^(#{1,6}\s)(.*)$/))) {
      out.push(span('mark', m[1]) + `<span class="md-heading">${inlineHTML(m[2])}</span>`)
    } else if (/^\s*([-*_])(\s*\1){2,}\s*$/.test(line)) {
      out.push(span('mark', line)) // --- rule
    } else if ((m = line.match(/^((?:\s*>)+\s?)(\[![\w-]+\][+-]?)?(.*)$/))) {
      out.push(span('quote', m[1]) + span('callout', m[2] ?? '') + inlineHTML(m[3]))
    } else if ((m = line.match(/^(\s*)([-*+]|\d+[.)])(\s+)(\[[ xX]\]\s)?(.*)$/))) {
      out.push(esc(m[1]) + span('list', m[2]) + esc(m[3]) + span('list', m[4] ?? '') + inlineHTML(m[5]))
    } else if (/^\s*\|/.test(line)) {
      out.push(inlineHTML(line).replace(/\|/g, '<span class="md-mark">|</span>'))
    } else {
      out.push(inlineHTML(line))
    }
  }
  // A last empty line still takes its height, as in the textarea.
  return out.join('\n') + '\n'
}

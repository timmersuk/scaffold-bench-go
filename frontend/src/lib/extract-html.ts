const VOID_ELEMENTS = new Set([
  "area", "base", "br", "col", "embed", "hr", "img", "input",
  "link", "meta", "param", "source", "track", "wbr",
]);

const OPEN_TAG_RE = /<([a-zA-Z][a-zA-Z0-9]*)\b[^>]*>/g;
const CLOSE_TAG_RE = /<\/([a-zA-Z][a-zA-Z0-9]*)\s*>/g;

export function extractHtml(output: string): string | null {
  const fencedMatch = output.match(/```(?:html)?\s*\n([\s\S]*?)\n```/);
  if (fencedMatch) {
    const html = fencedMatch[1].trim();
    if (looksLikeHtml(html)) return repairHtml(html);
  }

  const incompleteFenceMatch = output.match(/```(?:html)?\s*\n([\s\S]*)/);
  if (incompleteFenceMatch) {
    const html = incompleteFenceMatch[1].trim();
    if (looksLikeHtml(html)) return repairHtml(html);
  }

  const htmlTagMatch = output.match(/<html[\s\S]*<\/html>/i);
  if (htmlTagMatch) return htmlTagMatch[0].trim();

  const doctypeMatch = output.match(/<!doctype\s+html[\s\S]*<\/html>/i);
  if (doctypeMatch) return doctypeMatch[0].trim();

  const incompleteMatch = output.match(/(<!doctype\s+html|<html\b)([\s\S]*)/i);
  if (incompleteMatch) {
    const html = incompleteMatch[0].trim();
    if (looksLikeHtml(html)) return repairHtml(html);
  }

  return null;
}

function repairHtml(html: string): string {
  if (/<\/html\s*>/i.test(html)) return html;
  return closeOpenTags(html);
}

function closeOpenTags(html: string): string {
  const stack: string[] = [];

  let m: RegExpExecArray | null;
  const openRe = new RegExp(OPEN_TAG_RE.source, "g");
  while ((m = openRe.exec(html)) !== null) {
    const tag = m[1].toLowerCase();
    if (!VOID_ELEMENTS.has(tag)) {
      stack.push(tag);
    }
  }

  const closeRe = new RegExp(CLOSE_TAG_RE.source, "g");
  while ((m = closeRe.exec(html)) !== null) {
    const tag = m[1].toLowerCase();
    if (stack.length > 0 && stack[stack.length - 1] === tag) {
      stack.pop();
    }
  }

  for (let i = stack.length - 1; i >= 0; i--) {
    html += "</" + stack[i] + ">";
  }

  return html;
}

function looksLikeHtml(s: string): boolean {
  const lower = s.toLowerCase();
  return (
    lower.includes("<html") ||
    lower.includes("<!doctype") ||
    lower.includes("<head") ||
    lower.includes("<body")
  );
}

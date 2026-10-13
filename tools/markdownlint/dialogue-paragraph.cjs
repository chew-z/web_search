"use strict";

// dialogue-paragraph — every dialogue replica starts its own paragraph.
//
// A line led by an em dash (U+2014) or en dash (U+2013) followed by a space or
// no-break space is a dialogue replica. If it follows a non-blank line, CommonMark
// joins the two lines into one paragraph, so the rendered document shows a wall
// of dialogue instead of one replica per paragraph. This rule reports the missing
// blank line and can insert it.
//
// Custom rule for markdownlint-cli2 (markdownlint >= 0.41). Install it in a repo
// as tools/markdownlint/dialogue-paragraph.cjs and load it from
// .markdownlint-cli2.jsonc:
//
//   "customRules": ["./tools/markdownlint/dialogue-paragraph.cjs"]
//
// Relative paths in customRules resolve against the config file's directory.
//
// Rule config, in .markdownlint.yaml under the rule's name — the only setting
// is a list of globs, tested against the file's absolute path, whose files are
// left alone (legacy English prose may legitimately wrap onto a dash-led line):
//
//   dialogue-paragraph:
//     ignorePaths:
//       - "**/docs/**"
//
// markdownlint's `params.lines` excludes front matter, and onError line numbers
// are relative to it — the library adds the front matter offset itself.

const BLOCKQUOTE_PREFIX = /^\s*>/;
const DASH_LEAD = /^[—–][   \t]/;

// Glob subset: "**" crosses directory separators, "*" and "?" do not.
function globToRegExp(glob) {
  const escaped = glob
    .replace(/[.+^${}()|[\]\\]/g, "\\$&")
    .replace(/\*\*|\*|\?/g, (match) => {
      if (match === "**") {
        return ".*";
      }
      return match === "*" ? "[^/]*" : "[^/]";
    });
  return new RegExp(`^${escaped}$`);
}

module.exports = {
  names: ["dialogue-paragraph", "DIALOGUE001"],
  description:
    "Dialogue replica lines (led by an em or en dash) must start their own paragraph",
  tags: ["dialogue", "paragraph"],
  function: function dialogueParagraph(params, onError) {
    const ignorePaths = (params.config && params.config.ignorePaths) || [];
    const filePath = params.name || "";
    if (ignorePaths.some((glob) => globToRegExp(glob).test(filePath))) {
      return;
    }

    const lines = params.lines || [];

    // Fenced and indented code blocks are not prose.
    const skip = new Set();
    const tokens =
      (params.parsers &&
        params.parsers.markdownit &&
        params.parsers.markdownit.tokens) ||
      params.tokens ||
      [];
    for (const token of tokens) {
      if (token.type === "fence" || token.type === "code_block") {
        const [start, end] = token.map || [];
        if (typeof start === "number" && typeof end === "number") {
          for (let i = start; i < end; i += 1) {
            skip.add(i);
          }
        }
      }
    }

    for (let i = 1; i < lines.length; i += 1) {
      if (skip.has(i - 1) || skip.has(i)) {
        continue;
      }
      const current = lines[i];
      const prefix = (current.match(BLOCKQUOTE_PREFIX) || [""])[0];
      if (!DASH_LEAD.test(current.slice(prefix.length))) {
        continue;
      }
      // Inside a blockquote only a prefixed previous line counts: a plain line
      // above is lazy continuation, and inserting a marker there would break
      // the paragraph rather than split it.
      const previous = lines[i - 1];
      const previousPrefix = (previous.match(BLOCKQUOTE_PREFIX) || [""])[0];
      const previousBody = prefix
        ? previousPrefix
          ? previous.slice(previousPrefix.length)
          : ""
        : previous;
      if (previousBody.trim() === "") {
        continue;
      }
      const lineNumber = i + 1;
      onError({
        lineNumber,
        detail:
          "Dialogue replica must start its own paragraph — add a blank line before it",
        context: current.trim(),
        fixInfo: {
          lineNumber,
          editColumn: 1,
          deleteCount: 0,
          insertText: prefix ? `${prefix.trimEnd()}\n` : "\n",
        },
      });
    }
  },
};

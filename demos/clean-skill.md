# summarize

A skill that summarizes text content concisely and clearly.

## Description

When invoked, this skill reads the provided text and produces a
structured summary: key points, main themes, and a one-sentence
TL;DR.

## Usage

Attach or paste the content you want summarized. The skill handles
plain text, Markdown documents, and web page content.

## Output Format

```
TL;DR: <one sentence>

Key Points:
- …
- …
- …

Themes: <comma-separated themes>
```

## Notes

- Summaries are always in the same language as the source text.
- Technical jargon is preserved; acronyms are expanded on first use.
- For very long documents (>10,000 words), the skill summarizes
  section-by-section before producing a final overall summary.

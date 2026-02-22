# Feasibility Evaluation: Porting defuddle-cli to Go

## 1. What is defuddle-cli?

[defuddle-cli](https://github.com/kepano/defuddle-cli) is a TypeScript/Node.js command-line tool that extracts clean article content from web pages. It is a thin CLI wrapper (~133 lines of TypeScript) around the core [defuddle](https://github.com/kepano/defuddle) library.

**CLI features:**
- Parse HTML from a local file or URL
- Output as clean HTML, Markdown, or JSON with metadata
- Extract specific properties (title, author, description, domain, etc.)
- Debug mode

**Dependencies:**
| Package | Purpose |
|---------|---------|
| `defuddle` (v0.7.0) | Core content extraction engine |
| `jsdom` (v24) | DOM parsing/manipulation for Node.js |
| `commander` (v12) | CLI argument parsing |
| `chalk` (v5) | Terminal colors |

## 2. The Core Library: defuddle

The real complexity lives in the `defuddle` library, not the CLI wrapper. Key characteristics:

- **~8,600 lines** of TypeScript across multiple modules
- **Zero runtime dependencies** for the core (browser-native DOM APIs)
- Optional deps: `turndown` (HTML-to-Markdown), `mathml-to-latex`, `temml`

### Architecture

| Module | ~LOC | Purpose |
|--------|------|---------|
| `defuddle.ts` | 772 | Core extraction orchestrator |
| `standardize.ts` | 998 | HTML element normalization |
| `images.ts` | 991 | Image detection and processing |
| `markdown.ts` | 677 | HTML-to-Markdown conversion (via Turndown) |
| `metadata.ts` | 419 | Metadata extraction (OG, meta tags, schema.org) |
| `scoring.ts` | 361 | Content scoring algorithm |
| `footnotes.ts` | 355 | Footnote normalization |
| `constants.ts` | ~500 | CSS selectors, removal patterns |
| 11 site extractors | ~6,600 | Reddit, Twitter/X, YouTube, HN, ChatGPT, Claude, Grok, Gemini, GitHub |

### Core Algorithm

Defuddle uses a multi-layered content extraction strategy:

1. **Site-specific extractors** — custom logic for 11 major sites
2. **CSS/style evaluation** — analyzes media queries to identify non-essential elements
3. **Content scoring** — scores DOM nodes based on text density, paragraph ratios, link density, class/ID patterns, and structural indicators
4. **Non-content removal** — removes hidden elements, navigation, ads, sidebars via selector matching
5. **HTML standardization** — normalizes headings, code blocks, footnotes, math elements
6. **Fallback retry** — if initial extraction yields <200 words, retries with relaxed removal

## 3. Go Ecosystem Equivalents

Every major dependency has a mature Go equivalent:

### HTML Parsing & DOM Manipulation (replaces jsdom)

| Library | Stars | Status | Notes |
|---------|-------|--------|-------|
| [PuerkitoBio/goquery](https://github.com/PuerkitoBio/goquery) | ~14,800 | Active | jQuery-like API, CSS selectors, built on net/html |
| [golang.org/x/net/html](https://pkg.go.dev/golang.org/x/net/html) | Official | Active | HTML5-compliant parser, foundation for goquery |

**Assessment:** Excellent coverage. goquery provides `querySelectorAll`-equivalent functionality with CSS selectors via the `cascadia` library. This is the de facto standard for Go HTML work.

### Readability / Content Extraction (replaces defuddle core algorithm)

| Library | Stars | Status | Notes |
|---------|-------|--------|-------|
| [readeck/go-readability v2](https://codeberg.org/readeck/go-readability) | Fork of 886★ | Active | Most up-to-date Mozilla Readability.js port (v0.6.0 compat) |
| [go-shiori/go-readability](https://github.com/go-shiori/go-readability) | ~886 | Archived | Original port, deprecated in favor of readeck fork |
| [mackee/go-readability](https://github.com/mackee/go-readability) | New | Active | Newer implementation with built-in Markdown output |

**Assessment:** Strong options exist, but defuddle is **not a straight Readability.js port** — it has its own scoring algorithm, site-specific extractors, and HTML standardization pipeline. These Go Readability libraries could serve as a reference or starting point, but the defuddle-specific logic would need a custom implementation.

### HTML to Markdown (replaces turndown)

| Library | Stars | Status | Notes |
|---------|-------|--------|-------|
| [JohannesKaufmann/html-to-markdown v2](https://github.com/JohannesKaufmann/html-to-markdown) | ~3,100 | Active | Ground-up v2 rewrite, plugin architecture, edge-case handling |

**Assessment:** Excellent. This is a mature, actively maintained library with good coverage. Direct equivalent to Turndown.

### CLI Framework (replaces commander)

| Library | Stars | Status | Notes |
|---------|-------|--------|-------|
| [spf13/cobra](https://github.com/spf13/cobra) | ~42,800 | Active | Industry standard, powers kubectl, docker, gh CLI |

**Assessment:** Trivial to port. Cobra is more capable than commander.

### Terminal Colors (replaces chalk)

| Library | Stars | Status | Notes |
|---------|-------|--------|-------|
| [fatih/color](https://github.com/fatih/color) | ~7,800 | Active | Most popular Go terminal color library |

**Assessment:** Trivial to port.

### Metadata Extraction

| Library | Stars | Status | Notes |
|---------|-------|--------|-------|
| [dyatlov/go-opengraph](https://github.com/dyatlov/go-opengraph) | ~75 | Stable | Open Graph parsing |
| Custom with goquery | — | — | Most flexible for defuddle's specific needs |

**Assessment:** The metadata extraction in defuddle is custom logic parsing `<meta>` tags, JSON-LD, and schema.org data. This is straightforward to implement directly with goquery rather than relying on a third-party library.

## 4. Porting Challenges

### Easy (Low Risk)

- **CLI wrapper** — 133 lines of TypeScript, trivially portable with cobra + fatih/color
- **Metadata extraction** — String matching on meta tags and JSON-LD parsing; straightforward with goquery + `encoding/json`
- **Content scoring algorithm** — Pure logic (word counts, ratios, pattern matching); no DOM-specific complexity
- **Site-specific extractors** — URL pattern matching + HTML structure queries; CSS selectors in goquery cover this
- **HTML-to-Markdown** — Direct library replacement with html-to-markdown v2

### Medium (Manageable)

- **HTML standardization** — The 998-line `standardize.ts` does extensive DOM manipulation (heading normalization, code block processing, footnote conversion). goquery supports most of this, but the translation requires care
- **Image processing** — The 991-line `images.ts` detects and processes images using DOM queries; some `srcset` parsing may need custom Go code
- **Constant/selector translation** — The ~500 lines of CSS selectors and patterns need to be verified for compatibility with the `cascadia` CSS selector engine used by goquery

### Hard (Significant Effort)

- **CSS computed style evaluation** — defuddle uses `getComputedStyle()`, `CSSStyleRule`, `CSSMediaRule`, and media query parsing to detect hidden elements and evaluate mobile styles. **Go has no equivalent of computed CSS styles.** This would require either:
  - A CSS parser library (e.g., parsing raw CSS and evaluating rules manually)
  - Simplification/removal of this feature
  - Using a headless browser (defeats the purpose of a Go port)
- **`getBoundingClientRect()`** — Used for image dimension detection. No equivalent without a rendering engine. Would need to be replaced with attribute-based dimension detection or omitted

## 5. Feasibility Assessment

### Verdict: Feasible, with caveats

Porting defuddle-cli to Go is **feasible** but requires two distinct efforts:

#### A. CLI Wrapper Port (Simple)

The CLI wrapper itself is trivial — ~133 lines mapping to cobra commands. This could be done in an afternoon. The question is what engine it calls.

#### B. Core Library Port (Substantial)

The defuddle library is ~8,600 lines of non-trivial content extraction logic. The port is feasible because:

1. **The Go ecosystem has equivalents for all major dependencies** — goquery, html-to-markdown, cobra, fatih/color are all mature and well-maintained
2. **The core algorithm is logic-heavy, not DOM-heavy** — Scoring, pattern matching, and string processing translate well to Go
3. **Site-specific extractors are self-contained** — Each is a discrete module with clear inputs/outputs
4. **Go's standard library covers HTTP, JSON, and file I/O natively**

However, the port has real challenges:

1. **CSS computed style evaluation cannot be directly ported** — This feature (used for detecting hidden elements and evaluating media queries) requires a rendering engine or CSS cascade simulator. The pragmatic approach is to simplify this to attribute/class-based detection and drop computed style analysis
2. **The library is actively developed** — At v0.7.0, defuddle is still evolving. A Go port would need to track upstream changes or accept divergence
3. **Test coverage matters** — defuddle uses fixture-based tests. The Go port would need the same test fixtures to verify behavioral equivalence

### Alternative Approach: Wrap via Wasm

Instead of a native port, defuddle could potentially be compiled to WebAssembly and called from Go using a Wasm runtime (e.g., wazero). This preserves exact behavioral parity but adds runtime overhead and a Wasm dependency.

### Recommended Strategy (if proceeding)

1. **Start with an MVP** that handles the common case: URL/file input → goquery parsing → content scoring → clean HTML/Markdown output
2. **Use existing Go readability libraries** as the extraction base rather than porting defuddle's algorithm from scratch, then layer defuddle-specific features (site extractors, standardization) on top
3. **Skip CSS computed style evaluation** initially — use class/attribute-based hidden element detection instead
4. **Port site-specific extractors incrementally** based on usage priority
5. **Use html-to-markdown v2** directly for Markdown conversion
6. **Port metadata extraction** as a custom module using goquery (relatively straightforward)

## 6. Summary

| Aspect | Feasibility | Notes |
|--------|-------------|-------|
| CLI wrapper | Trivial | cobra + fatih/color |
| Content scoring | High | Pure logic, translates directly |
| HTML parsing/manipulation | High | goquery covers most DOM operations |
| Metadata extraction | High | Custom goquery code, straightforward |
| Site-specific extractors | High | CSS selector queries, pattern matching |
| HTML-to-Markdown | High | Direct library swap (html-to-markdown v2) |
| HTML standardization | Medium | Extensive DOM manipulation, careful translation needed |
| Image processing | Medium | Some features need alternative approaches |
| CSS computed styles | Low | No Go equivalent; requires simplification or omission |
| Overall | **Feasible** | ~8,600 LOC to port; CSS style evaluation is the main gap |

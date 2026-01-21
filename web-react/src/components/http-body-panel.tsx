import { json } from '@codemirror/lang-json';
import { html } from '@codemirror/lang-html';
import { javascript } from '@codemirror/lang-javascript';
import { xml } from '@codemirror/lang-xml';
import { yaml } from '@codemirror/lang-yaml';
import { defaultHighlightStyle, foldGutter, foldKeymap, syntaxHighlighting } from '@codemirror/language';
import { EditorState } from '@codemirror/state';
import { EditorView, keymap, lineNumbers } from '@codemirror/view';
import CodeMirror from '@uiw/react-codemirror';

export type BodyFormat = 'json' | 'html' | 'xml' | 'yaml' | 'javascript' | 'text' | 'sse';
export type BodyConfig = { value: string; format: BodyFormat };

const MAX_HIGHLIGHT_LENGTH = 200000;

const baseExtensions = [
  lineNumbers(),
  foldGutter(),
  keymap.of(foldKeymap),
  EditorView.lineWrapping,
  EditorView.editable.of(false),
  EditorState.readOnly.of(true),
  syntaxHighlighting(defaultHighlightStyle),
];

const getLanguageExtension = (format: BodyFormat) => {
  switch (format) {
    case 'json':
      return json();
    case 'html':
      return html();
    case 'xml':
      return xml();
    case 'yaml':
      return yaml();
    case 'javascript':
      return javascript();
    default:
      return null;
  }
};

const BodyViewer = ({ config }: { config: BodyConfig }) => {
  if (!config.value) {
    return (
      <div className="mt-1 rounded-md border border-border/60 bg-muted/30 p-2 text-xs text-muted-foreground">
        无正文
      </div>
    );
  }

  if (config.value.length > MAX_HIGHLIGHT_LENGTH) {
    return (
      <pre className="mt-1 max-h-48 overflow-auto whitespace-pre-wrap rounded-md border border-border/60 bg-muted/40 p-2 font-mono text-xs leading-relaxed text-foreground">
        {config.value}
      </pre>
    );
  }

  if (config.format === 'sse') {
    const lines = config.value.split(/\r?\n/);
    const sseKeys = new Set(['event', 'data', 'id', 'retry']);

    return (
      <pre className="mt-1 max-h-48 overflow-auto whitespace-pre-wrap rounded-md border border-border/60 bg-muted/40 p-2 font-mono text-xs leading-relaxed text-foreground">
        {lines.map((line, index) => {
          const lineKey = `${index}-${line}`;
          if (!line) {
            return <span key={lineKey}>{'\n'}</span>;
          }

          if (line.startsWith(':')) {
            return (
              <span key={lineKey} className="text-muted-foreground">
                {line}
                {'\n'}
              </span>
            );
          }

          const match = line.match(/^([^:]+):(.*)$/);
          if (!match) {
            return (
              <span key={lineKey} className="text-foreground">
                {line}
                {'\n'}
              </span>
            );
          }

          const key = match[1].trim();
          const value = match[2].replace(/^\s?/, '');

          return (
            <span key={lineKey}>
              <span className={sseKeys.has(key) ? 'text-primary' : 'text-foreground'}>{key}:</span>
              {value ? <span className="ml-1 text-foreground">{value}</span> : null}
              {'\n'}
            </span>
          );
        })}
      </pre>
    );
  }

  const language = getLanguageExtension(config.format);
  const extensions = language ? [...baseExtensions, language] : baseExtensions;

  return (
    <div className="mt-1 max-h-48 overflow-hidden rounded-md border border-border/60 bg-muted/40 text-xs">
      <CodeMirror
        value={config.value}
        extensions={extensions}
        editable={false}
        basicSetup={false}
        height="100%"
        maxHeight="12rem"
        className="text-xs"
      />
    </div>
  );
};

export function HttpBodyPanel({ title, config }: { title: string; config: BodyConfig }) {
  return (
    <div>
      <p className="text-xs font-semibold uppercase tracking-[0.2em] text-muted-foreground">{title}</p>
      <BodyViewer config={config} />
    </div>
  );
}

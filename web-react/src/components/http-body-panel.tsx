import { json } from '@codemirror/lang-json';
import { html } from '@codemirror/lang-html';
import { javascript } from '@codemirror/lang-javascript';
import { xml } from '@codemirror/lang-xml';
import { yaml } from '@codemirror/lang-yaml';
import { defaultHighlightStyle, foldGutter, foldKeymap, syntaxHighlighting } from '@codemirror/language';
import { EditorState } from '@codemirror/state';
import { EditorView, keymap, lineNumbers } from '@codemirror/view';
import CodeMirror from '@uiw/react-codemirror';

import { cn } from '@/lib/utils';

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

const BodyViewer = ({ config, className }: { config: BodyConfig; className?: string }) => {
  if (!config.value) {
    return (
      <div
        className={cn(
          'mt-1 flex min-h-0 flex-1 items-center justify-center rounded-md border border-border/60 bg-muted/30 p-2 text-xs text-muted-foreground',
          className
        )}
      >
        无正文
      </div>
    );
  }

  if (config.value.length > MAX_HIGHLIGHT_LENGTH) {
    return (
      <pre
        className={cn(
          'mt-1 min-h-0 flex-1 overflow-auto whitespace-pre-wrap rounded-md border border-border/60 bg-muted/40 p-2 font-mono text-xs leading-relaxed text-foreground',
          className
        )}
      >
        {config.value}
      </pre>
    );
  }

  if (config.format === 'sse') {
    const lines = config.value.split(/\r?\n/);
    const sseKeys = new Set(['event', 'data', 'id', 'retry']);

    return (
      <pre
        className={cn(
          'mt-1 min-h-0 flex-1 overflow-auto whitespace-pre-wrap rounded-md border border-border/60 bg-muted/40 p-2 font-mono text-xs leading-relaxed text-foreground',
          className
        )}
      >
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
    <div
      className={cn(
        'mt-1 min-h-0 flex-1 overflow-hidden rounded-md border border-border/60 bg-muted/40 text-xs',
        className
      )}
    >
      <CodeMirror
        value={config.value}
        extensions={extensions}
        editable={false}
        basicSetup={false}
        height="100%"
        className="h-full text-xs"
      />
    </div>
  );
};

export function HttpBodyPanel({
  title,
  config,
  className,
}: {
  title: string;
  config: BodyConfig;
  className?: string;
}) {
  return (
    <div className={cn('flex min-h-0 flex-col', className)}>
      <p className="text-xs font-semibold uppercase tracking-[0.2em] text-muted-foreground">{title}</p>
      <BodyViewer config={config} className="flex-1 min-h-0" />
    </div>
  );
}

import React from 'react';
import { Typography } from 'antd';

interface TextPreviewProps {
  content: string;
  fileName: string;
  truncated: boolean;
}

const LANGUAGE_MAP: Record<string, string> = {
  js: 'javascript',
  jsx: 'javascript',
  ts: 'typescript',
  tsx: 'typescript',
  py: 'python',
  rb: 'ruby',
  java: 'java',
  go: 'go',
  rs: 'rust',
  cpp: 'cpp',
  c: 'c',
  h: 'c',
  hpp: 'cpp',
  cs: 'csharp',
  php: 'php',
  swift: 'swift',
  kt: 'kotlin',
  scala: 'scala',
  html: 'html',
  css: 'css',
  scss: 'scss',
  less: 'less',
  json: 'json',
  xml: 'xml',
  yaml: 'yaml',
  yml: 'yaml',
  toml: 'toml',
  md: 'markdown',
  sql: 'sql',
  sh: 'bash',
  bash: 'bash',
  zsh: 'bash',
  dockerfile: 'dockerfile',
  conf: 'conf',
  ini: 'ini',
  cfg: 'ini',
  env: 'dotenv',
  txt: 'text',
  log: 'text',
};

function getLanguage(filename: string): string {
  const ext = filename.split('.').pop()?.toLowerCase() || '';
  const name = filename.toLowerCase();
  if (name === 'dockerfile') return 'dockerfile';
  if (name === 'makefile') return 'makefile';
  return LANGUAGE_MAP[ext] || 'text';
}

const TextPreview: React.FC<TextPreviewProps> = ({
  content,
  fileName,
  truncated,
}) => {
  const language = getLanguage(fileName);

  return (
    <div>
      {truncated && (
        <Typography.Text type="warning" style={{ display: 'block', marginBottom: 8 }}>
          File was truncated. Only the first 10MB is shown.
        </Typography.Text>
      )}
      <pre
        style={{
          background: '#f6f8fa',
          border: '1px solid #e1e4e8',
          borderRadius: 6,
          padding: 16,
          overflow: 'auto',
          maxHeight: 600,
          fontSize: 13,
          lineHeight: 1.5,
          fontFamily: "'SF Mono', 'Monaco', 'Inconsolata', 'Fira Code', monospace",
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-all',
        }}
      >
        <code className={`language-${language}`}>{content}</code>
      </pre>
    </div>
  );
};

export default TextPreview;

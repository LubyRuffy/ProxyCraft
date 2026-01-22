import { TrafficEntry } from '@/types/traffic';

export type TrafficTag = string;

const normalizeTags = (tags?: string[]) =>
  (tags ?? [])
    .map((tag) => tag.trim().toLowerCase())
    .filter((tag) => tag.length > 0);

const matchesAiTraffic = (entry?: TrafficEntry | null): boolean => {
  if (!entry) return false;
  const host = (entry.host ?? '').toLowerCase();
  const path = (entry.path ?? '').toLowerCase();
  const url = (entry.url ?? '').toLowerCase();

  if (host.includes('openai') || url.includes('openai')) return true;
  if (host.includes('anthropic') || host.includes('claude') || path.includes('/v1/messages') || path.includes('/v1/complete')) {
    return true;
  }
  if (host.includes('generativelanguage') || path.includes('generatecontent') || path.includes('streamgeneratecontent')) {
    return true;
  }
  if (host.includes('ollama') || path.includes('/api/generate') || path.includes('/api/chat')) return true;
  if (path.includes('/v1/chat/completions') || path.includes('/v1/completions') || path.includes('/v1/responses')) {
    return true;
  }
  if (path.includes('/backend-api/codex/responses')) return true;

  return false;
};

export const getTrafficTags = (entry?: TrafficEntry | null): TrafficTag[] => {
  if (!entry) return [];
  const tagSet = new Set<string>();

  if (entry.isHTTPS) tagSet.add('https');
  if (entry.isSSE) tagSet.add('sse');

  if (matchesAiTraffic(entry)) tagSet.add('ai');

  normalizeTags(entry.tags).forEach((tag) => {
    tagSet.add(tag);
  });

  return Array.from(tagSet);
};

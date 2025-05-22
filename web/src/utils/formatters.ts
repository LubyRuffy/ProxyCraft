import hljs from 'highlight.js/lib/core';
import json from 'highlight.js/lib/languages/json';
import 'highlight.js/styles/github.css';

// 只注册需要的语言以减小打包体积
hljs.registerLanguage('json', json);

/**
 * 检查字符串是否为JSON格式
 */
export const isJson = (str: string): boolean => {
  try {
    JSON.parse(str);
    return true;
  } catch (e) {
    return false;
  }
};

/**
 * 格式化并高亮JSON
 */
export const formatJson = (content: string | any): string => {
  try {
    // 如果输入是JSON字符串，先解析再格式化
    const jsonObj = typeof content === 'string' ? JSON.parse(content) : content;
    const formatted = JSON.stringify(jsonObj, null, 2);
    // 使用pre标签包装，确保正确应用hljs的样式
    const highlighted = hljs.highlight(formatted, { language: 'json' }).value;
    return `<code class="hljs language-json">${highlighted}</code>`;
  } catch (e) {
    console.error('JSON格式化失败:', e);
    return String(content);
  }
};

/**
 * 获取十六进制视图
 */
export const getHexView = (data: any): string => {
  if (!data) return '';

  // 将数据转换为字符串
  let str = typeof data === 'string' ? data : JSON.stringify(data);

  // 将字符串转换为UTF-8字节数组
  let bytes = new TextEncoder().encode(str);
  let result = '';

  // 每16个字节为一行
  for (let i = 0; i < bytes.length; i += 16) {
    // 添加偏移量
    result += (i).toString(16).padStart(8, '0') + ': ';

    let hexPart = '';
    let asciiPart = '';

    // 处理这一行的字节
    for (let j = 0; j < 16; j++) {
      if (i + j < bytes.length) {
        // 添加十六进制表示，保持两位宽度
        hexPart += bytes[i + j].toString(16).padStart(2, '0') + ' ';
        // 添加ASCII表示
        const byte = bytes[i + j];
        asciiPart += (byte >= 32 && byte <= 126) ? String.fromCharCode(byte) : '.';
      } else {
        // 用空格填充未满16字节的行
        hexPart += '   ';
      }
    }

    // 确保十六进制部分对齐，添加到结果中
    result += hexPart + ' ' + asciiPart;

    // 如果不是最后一行，添加换行符
    if (i + 16 < bytes.length) {
      result += '\n';
    }
  }

  return result;
};

/**
 * 获取状态码的CSS类
 */
export const getStatusClass = (statusCode: number): string => {
  if (statusCode >= 200 && statusCode < 300) {
    return 'status-success';
  } else if (statusCode >= 300 && statusCode < 400) {
    return 'status-redirect';
  } else if (statusCode >= 400 && statusCode < 500) {
    return 'status-client-error';
  } else if (statusCode >= 500) {
    return 'status-server-error';
  }
  return '';
};

/**
 * 获取状态码的文本描述
 */
export const getStatusText = (statusCode: number): string => {
  const statusTexts: Record<number, string> = {
    200: 'OK',
    201: 'Created',
    204: 'No Content',
    301: 'Moved Permanently',
    302: 'Found',
    304: 'Not Modified',
    400: 'Bad Request',
    401: 'Unauthorized',
    403: 'Forbidden',
    404: 'Not Found',
    500: 'Internal Server Error',
    502: 'Bad Gateway',
    503: 'Service Unavailable',
    504: 'Gateway Timeout'
  };

  return statusTexts[statusCode] || '';
};

/**
 * 格式化字节大小
 */
export const formatBytes = (bytes: number): string => {
  if (bytes === 0 || bytes === undefined) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
};

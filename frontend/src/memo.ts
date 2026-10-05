import DOMPurify from "dompurify";

export const sanitizeMemo = (html: string) =>
  DOMPurify.sanitize(html, {
    ALLOWED_URI_REGEXP:
      /^(?:(?:https?|file):|[^a-z]|[a-z+.\-]+(?:[^a-z+.\-:]|$))/i,
  });

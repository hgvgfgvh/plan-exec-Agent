/**
 * 轻量 Markdown 检测与安全渲染（WebUI 模型回复展示）。
 */
(function (global) {
  function escapeHtml(s) {
    return String(s)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  /** 启发式：是否像模型输出的 Markdown */
  function looksLikeMarkdown(text) {
    const t = String(text || "").trim();
    if (t.length < 12) return false;
    let score = 0;
    if (/^#{1,6}\s/m.test(t)) score += 2;
    if (/```[\s\S]*?```/.test(t)) score += 3;
    if (/^[-*+]\s+/m.test(t)) score += 1;
    if (/^\d+\.\s+/m.test(t)) score += 1;
    if (/\*\*[^*\n]+\*\*/.test(t)) score += 1;
    if (/^>\s/m.test(t)) score += 1;
    if (/^\|.+\|$/m.test(t) && /\|[\s\-:|]+\|/.test(t)) score += 2;
    if (/\|.+\|/.test(t) && /\|[-:\s|]{3,}\|/.test(t)) score += 2;
    if (/\n#{1,6}\s/.test(t)) score += 1;
    if (/`[^`\n]+`/.test(t) && (score > 0 || /^#{1,6}\s/m.test(t))) score += 1;
    return score >= 2;
  }

  function splitPlanFooter(text) {
    const re = /\n\n---\n(?:\s*\n)?（编排 /;
    const m = re.exec(text);
    if (m && m.index >= 0) {
      return { body: text.slice(0, m.index), footer: text.slice(m.index) };
    }
    return { body: text, footer: "" };
  }

  function safeLinkUrl(url) {
    const u = String(url || "").trim();
    if (/^(https?:\/\/|mailto:)/i.test(u)) return escapeHtml(u);
    return "";
  }

  function inlineFormat(escaped) {
    let s = escaped;
    s = s.replace(/`([^`\n]+)`/g, "<code>$1</code>");
    s = s.replace(/\*\*([^*\n]+)\*\*/g, "<strong>$1</strong>");
    s = s.replace(/\*([^*\n]+)\*/g, "<em>$1</em>");
    s = s.replace(/\[([^\]]+)\]\(([^)]+)\)/g, (_, label, url) => {
      const href = safeLinkUrl(url);
      if (!href) return label;
      return '<a href="' + href + '" target="_blank" rel="noopener noreferrer">' + label + "</a>";
    });
    return s;
  }

  function closeList(state, out) {
    if (!state.listType) return;
    out.push(state.listType === "ol" ? "</ol>" : "</ul>");
    state.listType = null;
  }

  /** 模型常把多行表格压成一行：| 列 | |---| | 行 | → 拆成多行（不拆表头单元格间的 | |） */
  function normalizeMarkdownInput(text) {
    let t = String(text || "").replace(/\r\n/g, "\n");
    t = t.replace(/\|\s+\|(?=\s*[-:])/g, "|\n|");
    t = t.replace(/\|\s+\|(?=\s*[A-Z][A-Z0-9]{0,6}[-‑]?\d)/g, "|\n|");
    return t;
  }

  function isTableLine(line) {
    const s = String(line || "").trim();
    if (!s.startsWith("|") || !s.endsWith("|")) return false;
    return (s.match(/\|/g) || []).length >= 2;
  }

  function parseTableCells(line) {
    const parts = String(line || "").trim().split("|");
    const cells = [];
    for (let i = 0; i < parts.length; i++) {
      const c = parts[i].trim();
      if (i === 0 && c === "") continue;
      if (i === parts.length - 1 && c === "") continue;
      cells.push(c);
    }
    return cells;
  }

  function isSeparatorRow(cells) {
    return (
      cells.length > 0 &&
      cells.every(function (c) {
        return /^:?-{1,}:?$/.test(c);
      })
    );
  }

  function renderTableBlock(tableLines) {
    const rows = tableLines.map(parseTableCells).filter(function (r) {
      return r.length > 0;
    });
    if (!rows.length) return "";

    let header = rows[0];
    let bodyRows = rows.slice(1);
    if (bodyRows.length && isSeparatorRow(bodyRows[0])) {
      bodyRows = bodyRows.slice(1);
    }

    const colCount = Math.max(
      header.length,
      bodyRows.reduce(function (m, r) {
        return Math.max(m, r.length);
      }, 0)
    );
    if (!colCount) return "";

    function padRow(cells, tag) {
      const out = [];
      for (let c = 0; c < colCount; c++) {
        const raw = cells[c] != null ? cells[c] : "";
        out.push(
          "<" + tag + ">" + inlineFormat(escapeHtml(raw)) + "</" + tag + ">"
        );
      }
      return "<tr>" + out.join("") + "</tr>";
    }

    const parts = ['<div class="md-table-wrap"><table class="md-table">'];
    parts.push("<thead>" + padRow(header, "th") + "</thead>");
    if (bodyRows.length) {
      parts.push("<tbody>");
      for (let i = 0; i < bodyRows.length; i++) {
        parts.push(padRow(bodyRows[i], "td"));
      }
      parts.push("</tbody>");
    }
    parts.push("</table></div>");
    return parts.join("");
  }

  function renderMarkdown(text) {
    const { body, footer } = splitPlanFooter(String(text || ""));
    const lines = normalizeMarkdownInput(body).split("\n");
    const out = [];
    const state = { listType: null, inCode: false, codeBuf: [] };

    function flushParagraph(buf) {
      const joined = buf.join(" ").trim();
      if (!joined) return;
      out.push("<p>" + inlineFormat(escapeHtml(joined)) + "</p>");
    }

    let paraBuf = [];

    for (let i = 0; i < lines.length; i++) {
      const line = lines[i];

      if (state.inCode) {
        if (/^```\s*$/.test(line.trimEnd()) || line.trim() === "```") {
          const code = escapeHtml(state.codeBuf.join("\n"));
          out.push("<pre><code>" + code + "</code></pre>");
          state.codeBuf = [];
          state.inCode = false;
        } else {
          state.codeBuf.push(line);
        }
        continue;
      }

      if (/^```/.test(line.trim())) {
        flushParagraph(paraBuf);
        paraBuf = [];
        closeList(state, out);
        state.inCode = true;
        state.codeBuf = [];
        const rest = line.trim().slice(3).trim();
        if (rest) state.codeBuf.push(rest);
        continue;
      }

      let trimmed = line.trim();

      if (!trimmed.startsWith("|")) {
        const mixed = trimmed.match(/^(.+?)\s+(\|.+)$/);
        if (mixed && isTableLine(mixed[2])) {
          flushParagraph(paraBuf);
          paraBuf = [];
          closeList(state, out);
          out.push("<p>" + inlineFormat(escapeHtml(mixed[1])) + "</p>");
          trimmed = mixed[2].trim();
        }
      }

      if (trimmed === "") {
        flushParagraph(paraBuf);
        paraBuf = [];
        closeList(state, out);
        continue;
      }

      const hm = trimmed.match(/^(#{1,6})\s+(.+)$/);
      if (hm) {
        flushParagraph(paraBuf);
        paraBuf = [];
        closeList(state, out);
        const level = Math.min(6, hm[1].length);
        out.push(
          "<h" +
            level +
            ">" +
            inlineFormat(escapeHtml(hm[2])) +
            "</h" +
            level +
            ">"
        );
        continue;
      }

      if (/^(-{3,}|\*{3,}|_{3,})$/.test(trimmed)) {
        flushParagraph(paraBuf);
        paraBuf = [];
        closeList(state, out);
        out.push("<hr />");
        continue;
      }

      const bq = trimmed.match(/^>\s?(.*)$/);
      if (bq) {
        flushParagraph(paraBuf);
        paraBuf = [];
        closeList(state, out);
        out.push("<blockquote><p>" + inlineFormat(escapeHtml(bq[1])) + "</p></blockquote>");
        continue;
      }

      const ul = trimmed.match(/^[-*+]\s+(.+)$/);
      if (ul) {
        flushParagraph(paraBuf);
        paraBuf = [];
        if (state.listType !== "ul") {
          closeList(state, out);
          out.push("<ul>");
          state.listType = "ul";
        }
        out.push("<li>" + inlineFormat(escapeHtml(ul[1])) + "</li>");
        continue;
      }

      const ol = trimmed.match(/^\d+\.\s+(.+)$/);
      if (ol) {
        flushParagraph(paraBuf);
        paraBuf = [];
        if (state.listType !== "ol") {
          closeList(state, out);
          out.push("<ol>");
          state.listType = "ol";
        }
        out.push("<li>" + inlineFormat(escapeHtml(ol[1])) + "</li>");
        continue;
      }

      if (isTableLine(trimmed)) {
        flushParagraph(paraBuf);
        paraBuf = [];
        closeList(state, out);
        const tableLines = [trimmed];
        let j = i + 1;
        while (j < lines.length) {
          const next = lines[j].trim();
          if (next === "" || !isTableLine(next)) break;
          tableLines.push(next);
          j++;
        }
        const tableHtml = renderTableBlock(tableLines);
        if (tableHtml) out.push(tableHtml);
        i = j - 1;
        continue;
      }

      closeList(state, out);
      paraBuf.push(trimmed);
    }

    if (state.inCode && state.codeBuf.length) {
      out.push("<pre><code>" + escapeHtml(state.codeBuf.join("\n")) + "</code></pre>");
    }
    flushParagraph(paraBuf);
    closeList(state, out);

    const html = out.join("\n") || "<p>" + inlineFormat(escapeHtml(body)) + "</p>";
    const footerHtml = footer ? escapeHtml(footer) : "";
    return { html, footerHtml };
  }

  global.MdRender = {
    escapeHtml,
    looksLikeMarkdown,
    renderMarkdown,
    splitPlanFooter,
  };
})(typeof window !== "undefined" ? window : globalThis);

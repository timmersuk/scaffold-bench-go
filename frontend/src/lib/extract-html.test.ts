import { describe, it, expect } from "vitest";
import { extractHtml } from "./extract-html";

describe("extractHtml", () => {
  describe("fenced code blocks", () => {
    it("extracts HTML from complete fenced block with html language", () => {
      const output = "Here's the code:\n```html\n<html><body>Hello</body></html>\n```\nDone.";
      const result = extractHtml(output);
      expect(result).toBe("<html><body>Hello</body></html>");
    });

    it("extracts HTML from complete fenced block without language", () => {
      const output = "```\n<!DOCTYPE html><html><head></head></html>\n```";
      const result = extractHtml(output);
      expect(result).toBe("<!DOCTYPE html><html><head></head></html>");
    });

    it("extracts HTML from incomplete fenced block", () => {
      const output = "```html\n<html><body>Test</body>";
      const result = extractHtml(output);
      expect(result).toBe("<html><body>Test</body></html>");
    });

    it("extracts HTML from incomplete fenced block without closing fence", () => {
      const output = "```\n<!doctype html><html><head><title>X</title></head><body></body>";
      const result = extractHtml(output);
      expect(result).toContain("<!doctype html>");
      expect(result).toContain("</body>");
    });
  });

  describe("unfenced HTML", () => {
    it("extracts HTML with <html> tags", () => {
      const output = "Some text\n<html><body>Content</body></html>\nMore text";
      const result = extractHtml(output);
      expect(result).toBe("<html><body>Content</body></html>");
    });

    it("extracts HTML with <!doctype>", () => {
      const output = "<!DOCTYPE html>\n<html>\n<head></head>\n<body></body>\n</html>";
      const result = extractHtml(output);
      expect(result).toContain("<!DOCTYPE html>");
      expect(result).toContain("</html>");
    });

    it("extracts incomplete HTML and repairs tags", () => {
      const output = "<html><body><div>Test";
      const result = extractHtml(output);
      expect(result).toContain("<html>");
      expect(result).toContain("<body>");
      expect(result).toContain("<div>");
      expect(result).toContain("</div>");
      expect(result).toContain("</body>");
      expect(result).toContain("</html>");
    });
  });

  describe("no HTML", () => {
    it("returns null when no HTML is found", () => {
      const output = "This is just plain text with no HTML tags.";
      const result = extractHtml(output);
      expect(result).toBeNull();
    });

    it("returns null for empty string", () => {
      const result = extractHtml("");
      expect(result).toBeNull();
    });

    it("returns null for whitespace only", () => {
      const result = extractHtml("   \n\n   ");
      expect(result).toBeNull();
    });
  });

  describe("HTML repair", () => {
    it("closes unclosed tags in correct order", () => {
      const output = "```html\n<html><body><div><span>Text</span></div>";
      const result = extractHtml(output);
      expect(result).toContain("</div>");
      expect(result).toContain("</body>");
      expect(result).toContain("</html>");
    });

    it("handles already complete HTML without adding tags", () => {
      const output = "<html><body>Complete</body></html>";
      const result = extractHtml(output);
      expect(result).toBe("<html><body>Complete</body></html>");
      expect(result).not.toContain("</html></html>");
    });

    it("does not close void elements", () => {
      const output = "<html><body><img src='x.png'><br><input type='text'></body></html>";
      const result = extractHtml(output);
      expect(result).not.toContain("</img>");
      expect(result).not.toContain("</br>");
      expect(result).not.toContain("</input>");
    });
  });

  describe("edge cases", () => {
    it("handles nested HTML structures", () => {
      const output = "```html\n<html><head><title>Test</title></head><body><div><p>Text</p></div></body></html>\n```";
      const result = extractHtml(output);
      expect(result).toContain("<title>Test</title>");
      expect(result).toContain("<p>Text</p>");
    });

    it("extracts HTML case-insensitively", () => {
      const output = "<HTML><BODY>Test</BODY></HTML>";
      const result = extractHtml(output);
      expect(result).toContain("<HTML>");
      expect(result).toContain("</HTML>");
    });

    it("handles HTML with attributes", () => {
      const output = "<html lang='en'><body class='main'><div id='content'>Test</div></body></html>";
      const result = extractHtml(output);
      expect(result).toContain("lang='en'");
      expect(result).toContain("class='main'");
      expect(result).toContain("id='content'");
    });
  });
});

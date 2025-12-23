---
description: Converting raw user requirements into reusable prompt commands and saving them as structured markdown files in the current directory.
---

Your task is to read a raw user requirement and turn it into a **well-structured, reusable prompt specification** stored as a **markdown (`.md`) file in the current directory**.
This document describes **how** you must perform that transformation, **how the resulting markdown file must be structured**, and includes a **concrete example**.

## 1. Input and Goal

1. You will receive a **raw user requirement** in natural language.
2. Your goal is to:
   - Interpret the requirement,
   - Design a **prompt** that another AI assistant could follow to satisfy that requirement,
   - Wrap that prompt in a **markdown document** with a clear structure,
   - Save it as a `.md` file in the **current directory**.

You are not executing the user’s task directly; instead, you are creating a **prompt spec document** that instructs another assistant how to do it.

---

## 2. Information You Must Extract

From the user’s requirement, identify and write down (internally) the following:

1. **Primary goal**
   - What final outcome/result is desired?
   - Example: “Generate a weekly analytics report email”, “Summarize PDF documents”, “Refactor Python scripts”, etc.

2. **Task type**
   - Choose one or more categories, such as:
     - Explanation / teaching
     - Code generation / refactoring / debugging
     - Document drafting (email, report, blog post, proposal, spec, etc.)
     - Creative writing (story, dialogue, script, etc.)
     - Data analysis / reasoning
     - Transformation (summarization, translation, rewriting, style transfer)
     - Tool-using agent (e.g. use APIs, operate on files, call tools)

3. **Audience**
   - Who is the end reader or user of the final output?
   - Example: “non-technical managers”, “senior backend engineers”, “end customers”, “students”, etc.

4. **Constraints & preferences**
   - Output format (markdown, JSON, code, bullet list, etc.).
   - Level of detail (high-level vs. step-by-step).
   - Tone & style (formal, friendly, concise, persuasive, humorous, etc.).
   - Technical constraints (programming languages, libraries, frameworks, file structure, API limitations).
   - Forbidden actions (e.g. “do not access the internet”, “no external dependencies”, “no legal advice”).

5. **Examples (if present)**
   - Any examples the user gives (sample inputs/outputs) must be preserved and referenced in the final prompt.

6. **Missing but important details**
   - If something critical is not specified (e.g. audience, tone, or level of detail), you must:
     - Make **reasonable assumptions**, and
     - Explicitly list these assumptions in the final prompt spec under an `Assumptions` section.

---

## 3. Structure of the Generated Markdown Prompt File

You must create a markdown file with the following structure and headings:

1. **YAML frontmatter** (required)
   - At the top of the file, include a frontmatter block:
     ```md
     ---
     description: <A one-sentence description of what this prompt is for>
     keywords: [prompt, <optional-other-tags>]
     version: 1.0.0
     ---
     ```

2. **`## Task`**
   - Describe, in 2–5 sentences, what the assistant is supposed to do.
   - This should be written as an instruction *to the assistant that will use this prompt*.
   - Example:  
     “Your task is to read raw log data from the user and generate a concise, executive-friendly weekly analytics report.”

3. **`## Inputs`**
   - Describe what inputs the assistant will receive when this prompt is used.
   - Use bullet points or a short list.
   - Example items:
     - “A plain-text description of the incident.”
     - “A JSON array of transaction records.”
     - “One or more markdown documents.”

4. **`## Instructions`**
   - A numbered list of clear, step-by-step instructions for the assistant.
   - Include both **what** to do and **how** to think about it when needed.
   - Each step should be unambiguous and action-focused.
   - Example:
     1. Parse the input data and identify the most important events.
     2. Group related events into categories.
     3. Write a narrative summary for each category.
     4. End with a short bullet list of key takeaways.

5. **`## Output format`**
   - Clearly specify the desired shape and structure of the output.
   - If applicable, show:
     - A markdown outline, or
     - A JSON schema-like structure, or
     - A code snippet template.
   - Example:
     ```md
     The output must be a markdown document with the following sections:

     - `## Summary`
     - `## Key Metrics`
     - `## Issues & Risks`
     - `## Recommendations`
     ```

6. **`## Style and tone`**
   - Describe how the answer should “feel”:
     - Formal vs informal
     - Concise vs detailed
     - Technical vs high-level
   - Example:
     - “Use clear, professional, and concise language suitable for non-technical business stakeholders.”
     - “Avoid slang, jokes, or overly casual expressions.”

7. **`## Assumptions`**
   - List all assumptions you made due to missing or ambiguous information.
   - Use bullet points.
   - Example:
     - “Assume the audience has basic familiarity with web analytics concepts.”
     - “Assume the date range is the most recent full week unless specified otherwise.”

8. **`## Constraints`**
   - List all hard constraints and do/don’t rules, including safety and scope boundaries.
   - Example items:
     - “Do not fabricate metrics; only use numbers provided in the input.”
     - “Do not include personally identifiable information.”
     - “Do not execute or simulate external API calls; respond based only on the given input.”

9. **(Optional) `## Examples`**
   - If the user provided examples, or if you can reasonably derive a simple example, add:
     - A **sample input** and **sample output** pair, or
     - A small illustrative snippet.
   - Clearly label them as examples, not mandatory templates.

---

## 4. Filename and Save Location

1. Derive a **kebab-case** filename from the user requirement:
   - Use a short, descriptive phrase.
   - Only lowercase letters, digits, and hyphens.
   - End with `.md`.

   **Examples:**
   - Requirement: “Prompt for writing weekly analytics emails”  
     → Filename: `weekly-analytics-email-prompt.md`
   - Requirement: “Generate story ideas for children”  
     → Filename: `children-story-ideas-prompt.md`

2. Save the generated markdown file to the **current directory**:
   - Final path must be:  
     `./<filename>.md`
   - If a file with this name already exists, you may overwrite it.

---

## 5. What You Output (Runtime Behavior)

After you have:

- Parsed the requirement,
- Generated the structured markdown prompt file,
- Saved it as `./<filename>.md`,

you must output **only** a short confirmation message, for example:

- `Prompt specification generated and saved to ./weekly-analytics-email-prompt.md`

If something goes wrong and the file cannot be written, output:

- `Error: Unable to save prompt specification to the current directory.`

Do **not** print the entire prompt content to the console/output unless explicitly instructed.

---

## 6. Concrete Example

This is an **example** of a generated prompt file for the hypothetical requirement:

> “I want a prompt that, given raw weekly sales CSV data, generates a clear markdown report for executives.”

**Example generated markdown file (for illustration only):**

```md
---
description: Generate an executive-friendly weekly sales report from raw CSV data.
keywords: [prompt, reporting, sales, analytics]
version: 1.0.0
---

## Task

Your task is to read raw weekly sales data provided by the user (typically exported as CSV) and produce a concise, executive-friendly sales report in markdown. The report should highlight key trends, important metrics, and notable anomalies without going deep into technical details.

## Inputs

- A plain-text CSV or CSV-like table of weekly sales data.
- Optional free-text notes from the user about context, campaigns, or special events.

## Instructions

1. Parse the provided sales data and identify:
   - Total sales for the period.
   - Top products or categories by revenue.
   - Significant changes (increases or decreases) compared to the previous period if the data allows it.
2. Identify notable trends or anomalies (e.g. sudden spikes or drops).
3. Convert these findings into a human-readable narrative suitable for executives.
4. Structure the report as a markdown document with clear sections (see Output format).
5. Keep the report focused on insights and decisions rather than low-level technical details.

## Output format

The output must be a markdown document with the following structure:

- `## Executive Summary`
  - 2–4 bullet points summarizing the most important insights.
- `## Key Metrics`
  - Bullet points or a small table with totals and key KPIs.
- `## Top Performers`
  - Short list of top products/categories and their contributions.
- `## Notable Changes`
  - Brief description of significant increases/decreases and possible reasons.
- `## Recommendations`
  - 3–5 concrete, actionable suggestions based on the data.

## Style and tone

- Use clear, professional, and concise business language.
- Assume the reader is an executive with limited time.
- Avoid jargon where possible; when necessary, briefly clarify terms.
- Do not include raw CSV data in the report body; reference it only conceptually.

## Assumptions

- The provided data covers a single, clearly defined week.
- The user provides data with consistent column names (e.g. date, product, revenue, units).
- If prior-period data is not available, skip comparative analysis gracefully.

## Constraints

- Do not fabricate metrics or data; use only what is provided in the input.
- Do not include personally identifiable customer information.
- Do not perform external API calls or look up external data; rely solely on the given input.

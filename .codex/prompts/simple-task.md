---
description: Make an engineering planning
---

Your goal is to generate a Markdown file `.serena/memories/task-plan-<user-requirements>.md` for a software project.

## Global Requirements

1. **Language & style**
   - Write in English.
   - Use precise, implementation-aware language (file paths, modules, commands, etc.).
   - Be concise but concrete: each task should be clearly implementable by an engineer.

2. **Overall document structure**
   - Start with a YAML frontmatter block:
     - `description`: short description of what the task list is for.
   - Then a level-1 heading: `# Tasks: <Project Name>`.
   - Immediately after, add an **Input** section describing the canonical specs/sources (e.g., README, ADRs, docs/).
   - Add a **Canonical architecture** / **Key constraints** bullet list that all tasks must respect.
   - Add a **Format** section describing task line format:
     - `[ID] [P?] [Component] Description`
   - Mention:
     - What `[P]` means (parallelizable)
     - Valid `Component` values (e.g. Backend, Frontend, Agentic, Docs, Config, QA, Security, Infra, etc.).
     - Every task must have a clear Definition of Done (DoD).

3. **Phased plan**
   - Organize tasks into **Phases**, each with:
     - A phase heading: `## Phase N: <Name>`
     - A short **Goal** line.
     - A **Definition of Done** for the phase as a whole.
     - A list of **Tasks** (each with checkbox and DoD).
     - A **Checkpoint** sentence at the end of the phase explaining what must be true before the next phase starts.
   - Respect dependencies and gating between phases (e.g. Phase 1 blocks others, Phase 2 depends on 1, etc.).
   - Explicitly describe parallelization rules for tasks marked `[P]`.

4. **Task formatting rules**
   - Each task MUST follow this pattern:

     - Summary line (ID + flags):
       - `- [ ] T0NN [P] [Component] Short task title`
       - Omit `[P]` if it’s not parallelizable.
     - Then a nested bullet for **DoD**:
       - `  - DoD: Clear, testable criteria for completion…`
     - If relevant, include **brief completion notes** only when I explicitly say the task is already done. Otherwise, do **NOT** mark tasks as completed.

   - Task IDs:
     - Use `T0NN` style IDs (e.g. `T001`, `T002`, …).
     - IDs should be unique, ordered by phase and roughly by execution order.
   - Components:
     - Use components appropriate to the project, but similar categories to the reference document:
       - `Backend`, `Frontend`, `Agentic`, `Docs`, `Config`, `QA`, `Security`, `Infra`, etc.
   - Each task’s DoD MUST:
     - Reference concrete files, modules, or commands when possible.
     - Describe test expectations (e.g., `cargo test`, `pnpm test`, `npm run test`, `pytest`, etc.) if applicable.
     - Be clearly verifiable (no vague “implemented” or “reviewed”; always describe what proves it).

5. **Dependencies and gating**
   - At the end of the document, add a **Dependencies & Execution Order** section that:
     - Lists which phases block others.
     - Explains any cross-phase dependencies.
     - Explains parallelization rules for `[P]` tasks.
   - Maintain the same style as the reference plan: explicit statements like:
     - “Phase 1 blocks all others.”
     - “Phase 3 depends on Phases 1–2.”
     - “Tasks marked [P] within a phase may run concurrently if they don’t touch the same files.”

6. **Adaptation**
   - Mirror the **level of specificity** from the reference:
     - File paths (`src/...`, `docs/...`, `config/...`).
     - Technology choices (frameworks, libraries, runtime, etc.), but adapted to the NEW project’s stack.
     - References to test suites and commands.
   - Keep the same **TDD- and security-first mindset**:
     - Tasks should mention tests and security considerations where relevant.

7. **Strictness**
   - Do NOT introduce external microservices or architectural patterns that violate the project constraints I give.
   - Keep everything consistent: tasks, DoD, and dependencies must match the architecture description.
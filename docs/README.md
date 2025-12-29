# QuantumLife Canon Documentation

**Company:** QuantumLayer Platform Ltd
**Product:** QuantumLife

---

## What Is This?

This folder contains the **QuantumLife Canon** — the constitutional document that defines what QuantumLife is and what it must never become.

This is not a PRD. Not an architecture document. Not a backlog.

This is a **meaning lock**.

---

## Contents

| File | Purpose |
|------|---------|
| `QUANTUMLIFE_CANON_V1.md` | The canonical definition of QuantumLife |
| `diagrams/` | Mermaid source files for all diagrams |

### Diagram Files

| File | Description |
|------|-------------|
| `diagrams/01_circles_intersections.mmd` | Overlapping sovereign circles and their intersections |
| `diagrams/02_root_circle_genesis.mmd` | How a root circle is created at onboarding |
| `diagrams/03_invite_flow.mmd` | Invite token acceptance sequence |
| `diagrams/04_intersection_contract.mmd` | Structure of an intersection contract |
| `diagrams/05_irreducible_loop.mmd` | The irreducible loop that all interactions follow |
| `diagrams/06_concentric_product_levels.mmd` | Single → Family → Enterprise → Market expansion |

---

## How to Use This Canon

### For Designers

Before designing any feature:

1. Read the canon
2. Identify which primitives apply
3. Sketch the solution using only canonical primitives
4. Verify against anti-drift rules
5. If you cannot map the feature to the canon, stop

### For Engineers

Before implementing anything:

1. Read the canon
2. Trace the implementation to a specific canon section
3. Use only the allowed primitives
4. Reject any forbidden concepts
5. If implementation cannot be traced to canon, do not build

### For AI Tools (Claude Code, etc.)

The **Execution Rule** applies:

> Claude Code and all AI tools implementing QuantumLife may build only what exists in this canon.

Before generating code:

1. Identify which canon primitive applies
2. Confirm the action stays within the irreducible loop
3. Verify no forbidden concepts are introduced
4. Check against anti-drift rules

If you cannot trace the request to this canon, refuse to implement.

---

## Anti-Drift Rules

### Forbidden Concepts at Core

These MUST NOT appear in any core implementation:

- **Users** — Only circles
- **Accounts** — Circles have identity
- **Roles** — Authority is explicit per intersection
- **Workspaces** — Intersections are the shared domains
- **Global State** — All state is owned by circles or intersections
- **Silent Permissions** — No permission expands without explicit approval

### QuantumLife MUST NOT Become

- Chatbot wrapper
- General app platform
- Social network
- Crypto playground
- Workflow engine
- Generic AI OS

### Hard Red Lines

1. **No global namespace** — If something exists globally, you're drifting
2. **No implicit permissions** — If access grows silently, you're drifting
3. **No platform lock-in** — If circles can't leave with their data, you're drifting
4. **No surveillance economics** — If the agent serves advertisers, you've failed
5. **No feature sprawl** — If you're adding features instead of circles, you're drifting

---

## Modifying the Canon

The canon is a constitutional document.

### Changes Require:

- Founder approval
- Clear justification
- Version bump
- Documented rationale

### Changes MUST NOT:

- Introduce forbidden concepts
- Violate the geometry
- Break the irreducible loop
- Enable drift toward forbidden patterns

### Change Process

1. Propose change with written rationale
2. Review against existing canon
3. Identify all downstream impacts
4. Obtain founder approval
5. Update version number
6. Document change in changelog

---

## Rendering Diagrams

The diagrams use Mermaid syntax. To render:

### In VS Code
Install the "Markdown Preview Mermaid Support" extension.

### In GitHub
Mermaid diagrams render natively in markdown files.

### Command Line
Use the Mermaid CLI:

```bash
npm install -g @mermaid-js/mermaid-cli
mmdc -i diagrams/01_circles_intersections.mmd -o diagrams/01_circles_intersections.svg
```

### In Documentation Sites
Most modern documentation generators (Docusaurus, MkDocs, GitBook) support Mermaid natively.

---

## Verification

To verify the canon is intact, check for required sections:

```bash
# Check for forbidden concepts section
grep -q "Forbidden at the Core" QUANTUMLIFE_CANON_V1.md && echo "OK: Forbidden concepts section present"

# Check for execution rule
grep -q "Execution Rule" QUANTUMLIFE_CANON_V1.md && echo "OK: Execution rule present"

# Check for anti-drift rules
grep -q "Anti-Drift Rules" QUANTUMLIFE_CANON_V1.md && echo "OK: Anti-drift rules present"

# Check for irreducible loop
grep -q "Irreducible Loop" QUANTUMLIFE_CANON_V1.md && echo "OK: Irreducible loop present"
```

---

## Quick Reference: The Primitives

| Primitive | One-Line Definition |
|-----------|---------------------|
| **Circle** | Sovereign agent with identity, memory, policy |
| **Intersection** | Versioned contract between circles |
| **Authority Grant** | Scoped delegation of capability |
| **Proposal** | Request to change intersection terms |
| **Commitment** | Binding agreement to act |
| **Action** | Executed operation within authority |
| **Settlement** | Confirmed completion |
| **Memory** | Private state owned by circle |

---

## Quick Reference: The Irreducible Loop

```
Intent → Intersection Discovery → Authority Negotiation → Commitment → Action → Settlement → Memory Update
```

This loop is the same at every scale: Single, Family, Enterprise, Market.

---

*This canon locks meaning. Drift is forbidden.*

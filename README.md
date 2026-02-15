# GitSense Chat CLI (gsc)

**The Intelligence Layer for Git: Turn any text file into a smart one.**

A CLI for querying repository intelligence and a framework for building AI-ready developer tools. Designed for humans and AI.

### The World's First

We believe this is the world's first intelligent, AI-auditable code repository. By publishing this repository with pre-built intelligence and auditable, AI-generated code, we are providing a template for how every repository can be made more human and AI-friendly. Our `.gitsense` directory is the new README, and we can demonstrate how today.

**Intelligent**

Most repositories are passive containers. They store the "how" (the code) but lose the "why" (the intent). The reasoning behind the architecture often remains trapped in documentation or the developer's head.

`gsc` makes your repository self-aware.

By simply chatting with an AI in [GitSense Chat](https://github.com/gitsense/chat), you create specialized 'Brains' that extract domain knowledge into manifest files, turning your repository into a queryable intelligence hub.

*   **For Humans:** No more guessing. Run `gsc brains` to see exactly what the repository knows.
*   **For AI:** No more blind spots. Use `gsc tree` to generate a metadata-enriched project map. This provides the agent with high-signal context while significantly reducing token usage.

 **AI Auditable**

Every line of AI-generated code that is used in `gsc` can be traced to a conversation, model and version. This repository demonstrates transparent AI development: 99.9% AI-generated, but manually guided through hundreds of messages to maintain architectural control. We have the receipts, and they are embedded directly in the code.

### The New Document

`gsc` is betting on a future where metadata becomes the new document. We are building `gsc` to make `.gitsense` as essential as a README. It is the bridge between human expertise and AI capability. By encoding domain knowledge as queryable metadata, we enable AI agents to discover code deterministically, not probabilistically. This is the paradigm shift that makes coding agents scalable.

The current building blocks are:
*   **`gsc brains`**: Loads architectural intent so agents can discover what the repository knows without reading text documents.
*   **`gsc query`**: Enables deterministic discovery by drilling into metadata, replacing the guesswork of keyword searching.
*   **`gsc grep`**: Finds code based on both content and intent, replacing blind grepping with context-aware search.
*   **`gsc tree`**: Visualizes a "Heat Map" of the project, replacing the need to browse raw file lists.
*   **`gsc insights`**: Provides a high-signal overview of the project, replacing the need to summarize long narrative documents.

These tools transform a passive repository into a natively AI-ready intelligence hub. **Try the Quick Start below to see these building blocks in action.**

### Requirements
*   **Git:** Required for repository info and finding the project root.
*   **Ripgrep:** Required in PATH for `gsc grep`. Ignore if not interested.

### Installation
Download a pre-compiled binary for Linux, macOS, or Windows from the releases page. Or if you prefer, you can build from source using the Go toolchain (version 1.21 or later required).

```bash
git clone https://github.com/gitsense/gsc-cli
cd gsc-cli
make build
alias gsc="$(pwd)/dist/gsc"
```

### Quick Start
Clone this repository if you have not already done so and run these commands to experience the "world's first?" self-aware codebase.

#### 1. Load the "Architect Brain"
Import the architectural intent of this project. 
```bash
gsc manifest import .gitsense/manifests/gsc-architect.json --name arch
```

Want to see how the brain was trained to think, take a look at `.gitsense/analyzers/gsc-architect.md`

#### 2. Discover Available Intelligence
List all available databases and fields to understand what intelligence is loaded in your workspace.
```bash
gsc brains
```

#### 3. Visualize the Intelligence Map
Stop guessing where logic lives. Use the `tree` command to see the repository's structure enriched with the "Why" behind every file.

```bash
# Show the purpose of files in the CLI and Logic layers, hiding everything else
gsc tree --db arch --fields purpose,layer --filter "layer in cli,internal-logic" --prune
```

Notice how `--prune` and `--filter` can be used to easily prune a 10,000+ file monorepo. 

#### 4. Search by Intent (Not Just Text)
Standard `grep` finds strings. `gsc grep` finds **context**. Find where "Execute" is called, but only within files the Architect Brain has tagged with the "bridge" topic.

```bash
gsc grep "Execute" --db arch --filter "topics=bridge" --fields purpose,topics
```

Note: You can define as many fields as you need to help identify connections and to refine/expand your search.

#### 5. Discover What the Repository Knows
If you're unsure what questions you can ask, get a distribution of the top values.

```bash
gsc insights --db arch --fields layer,topics --limit 20
```

Increase the limit to `1000` and add `--format json --code <gitsense chat code>` to send the output directly to GitSense Chat for AI-assisted analysis.

#### Next Steps

Run `gsc --examples` to view more examples.

### Built with AI, Designed by Humans
99.9% AI generated. 90% human architected with 0% Go knowledge. 

Is this code better than what a Go expert would write? Absolutely not. But it solves a real problem. We needed a way to provide portable binaries without forcing users to manage complex dependencies. Go was chosen because its long history provides a vast amount of training data, allowing AI to better assist in generating reliable code.

Can this code be maintained and evolved? We see no reason why not and we have the receipts to prove it. Every file is 100% traceable with a Block-UUID and version history. View the source and the version information to see what human guided AI can do. There is no guessing. We explicitly document which LLM generated each version of the code. 

For this initial release, we are not including the Git history and conversations that led to this "LLM version". Moving forward, our goal is to ensure that every feature and the conversations that created it can be easily reviewed.

### Code Provenance & Auditability

To truly make AI-assisted development maintainable, every source file embeds complete generation metadata:

**Dual-Versioning System:**
- **Product Version (e.g., v0.1.0):** Tracks functional releases.
- **LLM Version (e.g., v1.6.0):** Tracks iterative generation of each component.

**Traceability Fields:**
- `Block-UUID`: Unique identifier for the code block
- `Parent-UUID`: Chain of inheritance from previous versions
- `Authors`: Chronological record of LLM contributors with version numbers

This metadata provides deterministic traceability for AI-generated code, answering "what generated this, when, and why?"-a prerequisite for reviewing, debugging, and evolving AI-assisted systems. To learn more, try the interactive Traceable Code Demo in GitSense Chat.

### The Vision
**gsc scout** 

The end of blind grepping. While tools like `claude code` are powerful, they often rely on brute-force indexing and probabilistic guessing. Scout uses the intelligence layer to translate natural language intent into precise discovery loops. This saves money, improves context, and allows agents to work at scale without the token tax of guessing. 

Note: Scout is not about implementing code changes. Its sole purpose is to deliver the *right context* to AI agents as fast and as cheap as possible. We find the files; humans and AI coding agents does the work.

**gsc review**

The logical successor to discovery. Once `gsc scout` identifies the right context and changes are made, `gsc review` validates those changes against the repository's encoded intent. Most AI review tools are "diff-blind" - they see the code changes but don't understand the architectural impact. By leveraging the Brains imported via `gsc`, `gsc review` can answer: "Does this change violate the architectural layer definitions?" or "How does this modification affect the 'bridge' logic defined by the Architect Brain?" 

It transforms code review from a syntax check into an intent validation loop:

1. **Scout:** Find the context and intent.
2. **Change:** Human or AI implements the logic.
3. **Review:** Validate the change against the intent.
4. **Iterate:** Refine based on the architectural feedback,

**Tool Calling 2.0** 

We are moving from static metadata to executable intelligence. This is Tool Calling 2.0. Unlike traditional tool schemas (like MCP or Claude Tools) which are defined outside the codebase, `gsc` embeds tools and knowledge directly in your repository as queryable metadata. This makes `gsc` a perfect complement to MCP: while MCP standardizes the *how* of tool calling, `gsc` provides the *what* and *why* directly from the source of truth.

An agent can discover exactly what it is allowed to do by running:
```bash
gsc run --examples --format json
```

This returns a structured list of commands with descriptions and examples. The agent simply maps your natural language request to the best available command. This is discovery in its simplest form. You get the speed of an agent with the safety of a reviewed, deterministic contract, like a safeguarded delete:
```bash
gsc run guard-rm --db infrastructure --filter "protection=high"
```

This is still an idea, but we believe it's the future of agentic interaction. If you have a strong opinion on how metadata can make discovery easier and safer for AI, create an issue.

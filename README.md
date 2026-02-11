# GitSense Chat CLI (gsc)

**The Intelligence Layer for Codebase Discovery and Smarter Tools**

An execution engine for applying portable intelligence and a foundation for building smarter command line tools designed for humans and AI.

### The Problem
Without context, tools can only find what is written in a file, not what that file represents. Searching for text is not the same as searching for purpose, risk, or impact.

### The Solution
gsc makes your codebase smarter by applying intelligence created in GitSense Chat.

Domain experts use GitSense Chat to encode their expertise into specialized analyzers called Brains. These Brains can capture any metadata an expert deems important. This could be high-level business impact, security risks, or even mapping specific files to different documentation sets based on the target audience.

gsc imports this metadata and applies it to what are otherwise dumb text files. This transforms your repository into a queryable intelligence hub where you search for the specific insights an expert has defined. Instead of searching for text, you search for meaning.

### The World's First Intelligent Repository?
**No really!**

We believe this is the world's first intelligent repository, one that ships with its own intelligence manifest. We see metadata as the new documentation. It is queryable, multi-dimensional, and serves both humans and AI agents simultaneously.

For humans, this metadata provides a high-level map of intent. For AI agents, it is a high-fidelity sensory layer that enables zero-shot discovery. In the age of coding agents, we feel a `.gitsense` directory is as essential as a `README.md`. It can turn text into intent, allowing agents to operate at scale.

The included manifest was created by a GitSense Chat "Brain" designed to focus on project architecture. You can see exactly how this Brain thinks in `.gitsense/analyzers/gsc-architect.md`. This is the core of our "world's first" claim: the repository doesn't just store code, it stores the expertise required to understand it. You can create your own Brains for onboarding, security, or team ownership. Each one adds a new dimension of intelligence, and you can create and ship as many as you need.

Imagine you are an open source maintainer tired of answering the same questions repeatedly. You create a FAQ "Brain" by dumping your GitHub issues into a repository, import them into GitSense Chat, and have it analyze the patterns to create a queryable FAQ. This process can be automated to include new issues. As part of your contribution guidelines, you tell users that for the quickest response, they should try:

```bash
gsc query --db faq --field topics --insights --json
```

Feed the results into a chat and see if the FAQ guides can help. No human needed. No documentation to maintain. Just expertise, queryable and alive.

### Installation
Download a pre-compiled binary for Linux, macOS, or Windows from the releases page. Alternatively, build from source using the Go toolchain (version 1.21 or later required).

```bash
git clone https://github.com/gitsense/gsc-cli
cd gsc-cli
make build
alias gsc="$(pwd)/dist/gsc"
```

### Quick Start

Clone this repository, and try the following:

**Load the Lead Architect's Brain**
You are not just importing data. You are loading the architectural intent of the project as defined by a domain expert.
```bash
gsc manifest import .gitsense/manifests/gsc.json --name lead-architect
```

**Generate a High-Signal/Low-Token Map**
AI agents do not need to read 100 files to understand a project. This provides a structured JSON map of purpose and layer for every file, giving the agent instant context at a fraction of the token cost.
```bash
gsc tree --db lead-architect --fields purpose,layer --format json
```

**Semantic Discovery**
Stop guessing keywords. Find logic by its functional topic, such as persistence, to see exactly how the Lead Architect categorized the implementation.
```bash
gsc grep "sqlite" --filter "topic=persistence"
```

**The AI Bridge**
Seamlessly pipe this structured intelligence directly into your GitSense Chat session to ground your AI agent in deterministic facts.
```bash
gsc tree --db lead-architect --fields purpose --format json --code <your-6-digit-code>
```

This gives you a map of your repository's purpose and intent. Try achieving that with a README or standard grep. You can explore the meaning of the code before reading a single line of syntax.

### Built with AI, Designed by Humans
99.9% AI generated. 90% human architected with 0% Go knowledge. 

Is this code better than what a Go expert would write? Absolutely not. But it solves a real problem. We needed a way to provide portable binaries without forcing users to manage complex dependencies. Go was chosen because its long history provides a vast amount of training data, allowing AI to better assist in generating reliable code.

Can this code be maintained and evolved? We see no reason why not and we have the receipts to prove it. Every file is 100% traceable with a Block-UUID and version history. View the source and the version information to see what human guided AI can do. There is no guessing. We explicitly document which LLM generated each version of the code. 

For this initial release, we are not including the Git history that led to this "LLM version". Moving forward, our goal is to ensure that every feature and the conversations that created it can be easily reviewed.

### The CLI Bridge (Human-AI Partnership)
gsc was built to solve a specific problem: bringing high-signal codebase intelligence into the chat window. By using the --code flag, you can instantly bridge your terminal output to GitSense Chat.

```bash
# Send a JSON tree map to your chat session
gsc tree --code 123456 --db gsc --fields purpose --format json

# If the AI needs more context, send the full intelligence map
gsc query list --all --code 123456
```

This partnership allows humans to handle the high-level discovery while the AI focuses on implementation. While top models are capable, autonomous discovery often fails. gsc puts the human in control of the discovery loop, ensuring the AI always has the right context at the lowest possible cost.

### Core Features
Query the landscape, grep with semantic filters, and visualize the tree with metadata. These are the building blocks for smarter workflows.

### The Roadmap
**gsc scout**: An automated orchestrator that translates natural language intent into discovery loops. This will allow AI agents to find the exact files needed for a task without manual grepping, significantly reducing token waste.

**Smarter Tools**: A framework for custom, isolated commands. Imagine a version of rm that refuses to delete critical infrastructure files based on their metadata.

### Extensibility
Companies can build their own repository of commands. A single make build makes custom, metadata-driven tools available to the entire team.

### The Philosophy
Code is the how. Metadata is the why. Domain experts in GitSense Chat encode their tell-tale signs and domain knowledge into Brains. gsc applies that intelligence to make your tools smarter and your decisions safer.

### Requirements
*   **Ripgrep:** Required in PATH for search functionality.
*   **Git:** Required for repository info and finding the project root.

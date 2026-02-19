# Component: VHS Demo Script
# Block-UUID: 8068bdd5-f635-4a8f-899c-a2294f80877c
# Parent-UUID: 062fea0e-bd47-4dcd-9cf8-b632f4584ab4
# Version: 2.9.0
# Description: Refined demo script focusing on the "Brain" lifecycle and zero-shot discovery.
# Language: bash
# Created-at: 2026-02-16T17:47:17.228Z
# Authors: Gemini 3 Flash (v1.0.0), Gemini 3 Flash (v1.1.0), Gemini 3 Flash (v1.2.0), GLM-4.7 (v1.2.1), GLM-4.7 (v1.2.2), GLM-4.7 (v1.2.3), Gemini 3 Flash (v1.2.4), Gemini 3 Flash (v1.2.5), GLM-4.7 (v1.2.6), Gemini 3 Flash (v2.0.0), Gemini 3 Flash (v2.1.0), Gemini 3 Flash (v2.2.0), GLM-4.7 (v2.2.1), Gemini 3 Flash (v2.3.0), Gemini 3 Flash (v2.4.0), Gemini 3 Flash (v2.5.0), Gemini 3 Flash (v2.6.0), Gemini 3 Flash (v2.7.0), Gemini 3 Flash (v2.8.0), Gemini 3 Flash (v2.9.0)


Output docs/assets/demo.mp4
Set FontSize 18
Set Width 1200
Set Height 800
Set Padding 40
Set TypingSpeed 75ms

# --- Scene 1: The Problem & Strategic Solution ---
Sleep 1s
Type "# Coding agents like Claude Code brute-force discovery with ripgrep."
Sleep 1s
Enter
Type "# It works, but it's not scalable. The fix: self-aware repositories."
Sleep 1s
Enter
Enter

# --- Scene 2: The Intelligence Map (gsc brains) ---
Type "# To create a self-aware repository, you need two things:"
Sleep .5s
Enter
Type "#   - GitSense Chat to extract and create portable intelligence"
Enter
Sleep .5s
Type "#   - The `gsc` CLI to query that intelligence locally"
Sleep .5s
Enter
Enter
Type "# Since this repository is self-aware, let's see what it knows:"
Enter
Enter
Sleep 1s
Type "gsc brains"
Sleep 1s
Enter
Sleep 1.5s
Type "# Notice how it knows each file's `purpose` and `topic`."
Sleep 1.5s
Enter
Type "# Now let's see this knowledge in action to make Claude Code smarter!"
Sleep 1s
Enter
Enter

# --- Scene 3: Context-Aware Discovery ---
Type "# Let's search for 'Execute' but with context."
Sleep 1s
Enter
Type "# I'll limit the results to keep things concise."
Sleep 1s
Enter
Enter
Type "gsc grep 'Execute' --db arch --fields purpose,topics --limit 1"
Sleep 1s
Enter
Sleep 3s
Type "# Notice the 'purpose' and 'topics' metadata below the file."
Sleep 1.5s
Enter
Type "# These are just two of many fields available in the 'Architect' brain."
Sleep 1.5s
Enter
Type "# Agents can request exactly what they need, like 'public_api' or 'intent_triggers'."
Sleep 1.5s
Enter
Type "# This is how GitSense scales AI agents like Claude Code."
Sleep 1.5s
Enter
Enter
 
# --- Scene 4: Intent-Based Search ---
Type "# Now let's see how we can search even smarter to save money and time."
Sleep 1s
Enter
Type "# Context is powerful, but more context means more tokens."
Sleep 1s
Enter
Type "# Instead of just finding context, let's search with intent."
Sleep 1.5s
Enter
Enter
Type "gsc grep 'Execute' --db arch --fields purpose,topics --filter 'topics=bridge' --limit 1"
Sleep 1s
Enter
Sleep 3s
Type "# The --filter flag narrowed the search to only 'bridge' topics."
Sleep 1s
Enter
Type "# Now we're only seeing 'Execute' where it's actually acting as a bridge."
Sleep 2s
Enter
Enter
Type "# Imagine filtering by 'component' or 'team' to tame massive monorepos."
Sleep 1.5s
Enter
Type "# This is deterministic discovery: no guessing, just intent. This is GitSense."
Sleep 1.5s
Enter
Enter

# --- Conclusion ---
Type "# To learn more, try the Quick Start examples."
Enter
Sleep .5s
Type "# That's it and thanks for watching!"
Sleep 4s

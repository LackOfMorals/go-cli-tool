package skill

import _ "embed"

//go:generate go run ../../cmd/gen-skill

// SkillMD is the generated SKILL.md content embedded at build time. It is
// produced by cmd/gen-skill which walks the cobra command tree and appends
// the contents of skill-additions.md under a `## Gotchas` heading. The
// committed skill.md.gen is a placeholder until the generator runs.
//
//go:embed skill.md.gen
var SkillMD []byte

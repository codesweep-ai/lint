package lint

import _ "embed"

// The governance files a published project carries verbatim, compiled into the
// binary so a check has something to compare against on a machine with no
// checkout and no network.
//
// They are this repository's own copies rather than a second set kept for the
// purpose. A reference kept beside the checks is a reference nobody reads, and
// it drifts from the file the project actually ships; embedding the shipped
// file means the tool cannot pass a repository whose governance differs from
// its own.
var (
	// LicenceText is LICENSE: the Apache 2.0 text, unmodified, placeholders and
	// all. The appendix is instructional boilerplate rather than a grant, so
	// filling it in edits the licence to say something the canonical text does
	// not. The copyright holder is named in NOTICE instead.
	//
	//go:embed LICENSE
	LicenceText string

	// NoticeText is NOTICE, whose second line names the copyright holder. Only
	// that line is shared: the first names the project, which differs by
	// repository.
	//
	//go:embed NOTICE
	NoticeText string

	// CodeOfConductMD is CODE_OF_CONDUCT.md: Contributor Covenant 2.1 with the
	// reporting address filled in and the attribution block intact. The text is
	// under CC BY 4.0, which a paraphrase that drops the attribution does not
	// satisfy, and a shortened copy loses the enforcement ladder that makes the
	// document act on anything.
	//
	//go:embed CODE_OF_CONDUCT.md
	CodeOfConductMD string
)

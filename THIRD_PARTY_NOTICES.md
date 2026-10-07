# Third-party data

QueueAtlas remains licensed under MIT.

`internal/auth/password_blocklist.sha256` contains a filtered, normalized and
SHA-256-transformed subset of SecLists' public password list. SecLists is licensed
under MIT, copyright (c) 2018 Daniel Miessler. Its complete notice is retained in
[password_blocklist.LICENSE](internal/auth/password_blocklist.LICENSE).

Source revision, attribution, checksums, transformation and maintenance are in
[the corpus documentation](docs/password-blocklist.md). Distributions containing
this embedded data must include this notice and the retained SecLists license,
alongside QueueAtlas' own license. Native package assembly remains planned for M5.

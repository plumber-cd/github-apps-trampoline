# github-apps-trampoline

A cross-platform no-dependency GIT_ASKPASS trampoline for GitHub Apps, written in Go.

## Purpose

This is inspired by https://github.com/desktop/askpass-trampoline project.

If you already know what [GitHub Apps](https://docs.github.com/en/developers/apps/getting-started-with-apps/about-apps#about-github-apps) is - you probably also know common challenges to work with them, namely - it's a bit of work to convert app credentials into a username+token pair that could be used by a regular Git clients such as Git CLI and various Git SDKs.

If you also know about [Git AskPass Credentials Helpers](https://git-scm.com/docs/gitcredentials) -  your already know that there is actually an easy solution to these challenges. You can do something like `git config --global credential.helper "/usr/bin/my-helper"` and that way you can easily delegate to external program the process to generate JWT and request a temporary token with it. With a slight problem - you have to create that external program.

Yeah, that was a bit of a bummer. Almighty GitHub did not provide us with and "official" one, but oh well - that is not a rocket science and we can do it ourselves. This project is exactly that - an open source community-maintained configurable helper that can use your SSH Private Key to generate JWT and then request a temporary token with it. You don't have to create it - just configure it and make your Git client to use it.

It can also be used as a standalone CLI app so it can be embedded into workflows as a little helper to request temp credentials.

If you happen to write your workflows in Go - you could also use packages from this repository as dependencies to build your own.

## Installation

TBD

## Usage

TBD

```bash
# To use as a helper, configure your Git Client as follows
git config --global credential.useHttpPath true
git config --global credential.helper "/path/to/github-apps-trampoline -c /path/to/config.json"

# Configurable via JSON as AskPass Helper
# Using full JSON config will always supersede any CLI arguments
cat < EOF > config.json
{
    "github\\.com/foo/bar": {
        "key": "private.key",
        "permissions": {"contents": "write"},
        "current_repo": true
    },
    "github\\.com/foo/.*": {
        "key": "private.key",
        "permissions": {"contents": "read"}
    },
    ".*": {
        "key": "private.key",
        "installation_id": "<numeric ID of the installation - if no provided will automatically infer from the current repo>",
        "installation": "<alternatively - installation path such as github.com/foo>",
        "repository_ids": "<optional XXX,YYY>",
        "repositories": "<optional foo,bar>",
        "permissions": {"contents": "read"}
    }
}
EOF
github-apps-trampoline -c config.json
GITHUB_APPS_TRAMPOLINE="$(cat config.json)" github-apps-trampoline
GITHUB_APPS_TRAMPOLINE_CONFIG="config.json" github-apps-trampoline

# Somewhat configurable via CLI as AskPass Helper
# This will generate config.json in-memory with a single key
# Some of these examples are not secure to use:
#   missing --permissions will assume all permissions from the app scope
#   missing --repository-ids, --repositories and --current-repo will assume access to all repositories in the current installation
github-apps-trampoline --key private.key --filter 'github\.com/foo/bar' --current-repo=true --permissions '{"contents": "write"}'
github-apps-trampoline --key private.key --filter 'github\.com/foo/.*' --permissions '{"contents": "read"}'
github-apps-trampoline --key private.key # using no --filter is the same as using --filter '.*'
github-apps-trampoline --key private.key --permissions '{"contents": "read"}'
github-apps-trampoline --key private.key --repository-ids 'XXX,YYY'
github-apps-trampoline --key private.key --repositories 'bar,baz'
github-apps-trampoline --key private.key --installation-id 'XXX'
github-apps-trampoline --key private.key --installation 'github.com/foo'

# As a standalone CLI
# It will spit out JSON in STDOUT
# It will not read STDIN and will not use config.json - it will only use the CLI input
github-apps-trampoline --cli --key private.key --installation 'github.com/foo' --repositories 'bar,baz' --permissions '{"contents": "read"}'
```

Enabling verbose mode will print credentials in STDERR - use with caution.

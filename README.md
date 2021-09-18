# github-apps-trampoline

A cross-platform no-dependency GIT_ASKPASS trampoline for GitHub Apps, written in Go.

## Purpose

This is inspired by https://github.com/desktop/askpass-trampoline project.

If you already know what [GitHub Apps](https://docs.github.com/en/developers/apps/getting-started-with-apps/about-apps#about-github-apps) means - you probably also know common challenges to work with them, namely - it's a bit of work to convert app credentials into a username+token pair that could be used by a regular Git clients such as Git CLI and various SDKs.

If you also know about [Git AskPass Credentials Helpers](https://git-scm.com/docs/gitcredentials) - there is actually an easy solution to this problem. You can do something like `git config --global credential.helper "/usr/bin/my-helper"` and that way you can easily delegate to external program that process to generate JWT and request a temporary token with it. With a slight problem - the helper you have to create yourself!

Yeah that was a bit of a bummer almighty GitHub did not provide us with one, but oh well - that is not a rocket science and we can do it ourselves. This project is exactly that - a configurable helper that can use your SSH Private Key to generate JWT and then request a temporary token with it. You don't have to create it - just configure with environment variables or a JSON.

## Installation

## Usage

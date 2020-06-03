# Git LFS: Shared Folder agent

[![Build status][1]][2]

[1]: https://travis-ci.org/sinbad/lfs-folderstore.svg?branch=master
[2]: https://travis-ci.org/sinbad/lfs-folderstore

## What is it?

`lfs-folderstore` is a [Custom Transfer
Agent](https://github.com/git-lfs/git-lfs/blob/master/docs/custom-transfers.md)
for [Git LFS](https://git-lfs.github.com/) which allows you to use a plain
folder as the remote storage location for all your large media files.

## Why?

Let's say you use Git, but you don't use any fancy hosting solution. You just
use a plain Git repo on a server somewhere, perhaps using SSH so you don't even
need a web server. It's simple and great.

But how do you use Git LFS? It usually wants a server to expose API endpoints.
Sure you could use one of the [big](https://bitbucket.org) [hosting](https://github.com)
[providers](https://gitlab.com), but that makes everything more complicated.

Maybe you already have plenty of storage sitting on a NAS somewhere, or via
Dropbox, Google Drive etc, which you can share with your colleagues. Why not just
use that?

So that's what this adapter does. When enabled, all LFS uploads and downloads
are simply translated into file copies to/from a folder that's visible to your
system already. Put your media on a shared folder, or on a synced folder like
Dropbox, or Synology Cloud Drive etc.

## How to use

### Prerequisites

You need to be running Git LFS version 2.3.0 or later. This has been tested
with 2.5.2 and 2.6.0 (and with Git 2.19.1).

### Download &amp; install

You will need `lfs-folderstore[.exe]` to be on your system path somewhere.

Either download and extract the [latest
release](https://github.com/sinbad/lfs-folderstore/releases), or build it from
source using the standard `go build`.

### Configure a fresh repo

Starting a new repository is the easiest case.

* Initialise your repository as usual with `git init` and `git lfs track *.png` etc
* Create some commits with LFS binaries
* Add your plain git remote using `git remote add origin <url>`
* Run these commands to configure your LFS folder:
  * `git config --add lfs.customtransfer.lfs-folder.path lfs-folderstore`
  * `git config --add lfs.customtransfer.lfs-folder.args "C:/path/to/your/folder"`
  * `git config --add lfs.standalonetransferagent lfs-folder`
* `git push origin master` will now copy any media to that folder

A few things to note:

* As shown, if on Windows, use forward slashes for path separators
* If you have spaces in your path, add **additional single quotes** around the path
    * e.g. `git config --add lfs.customtransfer.lfs-folder.args "'C:/path with spaces/folder'"`
* The `standalonetransferagent` forced Git LFS to use the folder agent for all
  pushes and pulls. If you want to use another remote which uses the standard
  LFS API, you should see the next section.

### Configure an existing repo

If you already have a Git LFS repository pushing to a standard LFS server, and
you want to either move to a folder, or replicate, it's a little more complicated.

* Create a new remote using `git remote add folderremote <url>`. Do this even if you want to keep the git repo at the same URL as currently.
* Run these commands to configure the folder store:
  * `git config --add lfs.customtransfer.lfs-folder.path lfs-folderstore`
  * `git config --add lfs.customtransfer.lfs-folder.args "C:/path/to/your/folder"`
  * `git config --add lfs.<url>.standalonetransferagent lfs-folder` - important: use the new Git repo URL
* `git push folderremote master ...` - important: list all branches you wish to keep LFS content for. Only LFS content which is reachable from the branches you list (at any version) will be copied to the remote

### Cloning a repo

There is one downside to this 'simple' approach to LFS storage - on cloning a
repository, git-lfs can't know how to fetch the LFS content, until you configure
things again using `git config`. That's the nature of the fact that you're using
a simple Git remote with no LFS API to expose this information.

It's not that hard to resolve though, you just need a couple of extra steps
when you clone fresh. Here's the sequence:

* `git clone <url> <folder>`
    * this will work for the git data, but will report "Error downloading object" when trying to get LFS data
* `cd <folder>` - to enter your newly cloned repo
* Configure as with a new repo:
  * `git config --add lfs.customtransfer.lfs-folder.path lfs-folderstore`
  * `git config --add lfs.customtransfer.lfs-folder.args "C:/path/to/your/folder"`
  * `git config --add lfs.standalonetransferagent lfs-folder`
* `git reset --hard master`
  * This will sort out the LFS files in your checkout and copy the content from the now-configured shared folder

## Notes

* The shared folder is, to git, still a "remote" and so separate from clones. It
  only interacts with it during `fetch`, `pull` and `push`.
* Copies are used in all cases, even if you're using Dropbox, Google Drive etc
  as your folder store. While hard links are possible and would save space, for
  integrity reasons (no copy-on-write) I've kept things simple.
* It's entirely up to you whether you use different folder paths per project, or
  share one between many projects. In the former case, it's easier to reclaim
  space by deleting a specific project, in the latter case you can save space if
  you have common files between projects (they'll have the same hash)

## License (MIT)

Copyright © 2018 Steve Streeting

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.

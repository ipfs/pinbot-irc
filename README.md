# pinbot-irc

[![](https://img.shields.io/badge/made%20by-Protocol%20Labs-blue.svg?style=flat-square)](http://ipn.io)
[![](https://img.shields.io/badge/project-IPFS-blue.svg?style=flat-square)](http://ipfs.io/)
[![](https://img.shields.io/badge/freenode-%23ipfs-blue.svg?style=flat-square)](http://webchat.freenode.net/?channels=%23ipfs)
[![standard-readme compliant](https://img.shields.io/badge/standard--readme-OK-green.svg?style=flat-square)](https://github.com/RichardLitt/standard-readme)

> a bot for the ipfs irc channel that pins things (among other menial tasks)

IRC bot that pins [IPFS](http://ipfs.io) files.

## Install

```sh
go get -u github.com/whyrusleeping/pinbot
```

## Usage

In a terminal, run:

```sh
pinbot [-s <server>]
```

In IRC:

```irc
<jbenet> !friends
<pinbot> my friends are: whyrusleeping lgierth jbenet
<jbenet> !pin QmbTdsZpRdVC7au7jLtkMwD6PRJPvfPvdRzG817PnxR2pR
<pinbot> now pinning QmbTdsZpRdVC7au7jLtkMwD6PRJPvfPvdRzG817PnxR2pR
<pinbot> pin QmbTdsZpRdVC7au7jLtkMwD6PRJPvfPvdRzG817PnxR2pR successful! -- http://gateway.ipfs.io/ipfs/QmbTdsZpRdVC7au7jLtkMwD6PRJPvfPvdRzG817PnxR2pR
<jbenet> !botsnack
<pinbot> om nom nom
```

Make sure to change the friends array. (or bug us to make this better configurable in an issue)

## Contribute

Feel free to join in. All welcome. Open an [issue](https://github.com/ipfs/pinbot-irc/issues)!

This repository falls under the IPFS [Code of Conduct](https://github.com/ipfs/community/blob/master/code-of-conduct.md).

[![](https://cdn.rawgit.com/jbenet/contribute-ipfs-gif/master/img/contribute.gif)](https://github.com/ipfs/community/blob/master/contributing.md)

## License

[MIT](LICENSE)

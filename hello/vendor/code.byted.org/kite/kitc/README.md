# kitc

[![Go Report Card](http://golang-report.byted.org/badge/code.byted.org/kite/kitc)](http://golang-report.byted.org/report/code.byted.org/kite/kitc)
[![build status](https://code.byted.org/kite/kitc/badges/master/build.svg)](https://code.byted.org/kite/kitc/commits/master)
[![coverage report](https://code.byted.org/kite/kitc/badges/master/coverage.svg)](https://code.byted.org/kite/kitc/commits/master)

# 贡献指南

kite框架下的项目的代码库都会有master和develop两个分支。master分支作为线上分支，提供`go get`方法获取。master分支的代码只能是通过develop分支合并进去，不可以直接在master分支上开发，只有`hot fix`的可以直接合并代码到master分支。

### 提交代码过程

* 项目贡献者在项目的issue页发起一个新的issue，描述给项目增加新功能的原因，以及准备要怎么做，在issue下面和项目负责人讨论，达成一致后开始开发。
* 项目贡献者将自己本地的develop分支更新到最新，从develop分支检出一个新的分支，新分支的名字最好和需要开发的功能有直接关联，然后基于新的分支开发功能。
* 新功能开发需要事先设计好，如果新添加功能的代码变动比较多，需要设计分多次提交MR，保证单次MR的diff代码行数在100行上下。
* 对新增代码添加单元测试，本地测试没有问题之后，提交修改，并且将该新分支推到远程代码库，此时会触发自动单元测试，确认单元测试没有任何异常。
* 在MR页面上向develop分支发起Merge Request请求，在MR中引用之前提出的issue链接，并且对新加功能做简单的语言描述，加快CR人员的理解。
* CR人员对代码改动有疑问的，在对应的代码位置给出问题描述，如果需要改进，代码贡献者针对当前分支代码进行修改，然后在推到远程分支，直到最后合并进develop分支为止。


### hotfix代码

如果因为master分支上有明显的错误，那么直接从master分支检出一个hotfix分支，在该分支上开发功能，并且最终向master分支提出MR


### 代码规范


Go语言的编码规范，参照两篇官方的编码规范描述

* [CodeReviewComments](https://github.com/golang/go/wiki/CodeReviewComments)
* [commentary](https://golang.org/doc/effective_go.html#commentary)


### 问题报告

如果在使用kite框架的过程中发现kite框架相关的问题，可以直接在相关项目的issue页面发起一个issue，详细描述问题内容，如果必要，附上相应的错误信息。

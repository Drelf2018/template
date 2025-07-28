### 是什么

这是一个利用 DeepSeek 评论指定用户最新一条微博的模板。

### 怎么用

```
> go install github.com/Drelf2018/template/cmd/template@latest
> template automatic_comment.yml --cookie="a1" --key="a2" --uid=a3
```

`a1` 为发送评论的账号的 `cookie`

`a2` 是用来调用 `DeepSeek` 的 `api_key`

`a3` 是要评论的博主的 `UID`
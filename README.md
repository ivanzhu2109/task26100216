# NimbusSync 离线目录移动合并库

Go 实现的内存态、单线程目录移动合并引擎。目录集合固定：0 为根，1..N 为普通目录。

## 语义

- 每条移动事件为 `(folder, parent, priority)`；priority 全局唯一（重复投递的事件三字段完全一致），事件可乱序、重复到达。
- 每个目录只保留已接收事件中 priority 最大的一条作为候选移动；无事件时候选父目录为根。
- 展示关系 = 候选关系中每个有向环恰好把 priority 最小的边临时改指根，其余边不变；自指是长度为 1 的环。展示关系恒为以 0 为根的无环树。
- 被压制的移动仍是候选；环被后续事件拆散时自动恢复生效。

## API

```go
s := nimbussync.New(n)
s.Apply(folder, parent, priority)
p := s.Parent(folder)
snap := s.Snapshot() // map[folder]parent，覆盖 1..n
```

## 验证

```sh
go test ./...
```

包含：环断裂/自动恢复/旧事件与重复事件不倒退/入环边不误切的确定性场景，以及随机事件对拍慢速参考实现、乱序加重复投递的最终 Snapshot 一致性测试。

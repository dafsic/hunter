## 流程图
![flow](/strategy/assets/test.png)


## 伪代码
```
行情分析函数 () {
	for {
        select {
            case 资金费更新：
                计算综合日化，更新做单方向。
            case 现货最优挂单更新：
                更新现货最优挂单
            case 合约最优挂单更新：
                更新合约最优挂单
            case 终止信号：
                退出
        }
    
        if 合约bid > 现货bid && 做单方向 != 空头 {
            select { // 无缓冲channel,加default,阻塞就丢弃
                case 现货建议下单channel <- {价格: (1-阈值) * 合约bid, 方向: 买入}:
                default: 
            }
        }

        if 合约ask < 现货ask && 做单方向 != 多头 {
            select { // 无缓冲channel,加default,阻塞就丢弃
                case 现货建议下单channel <- {价格: (1+阈值) * 合约ask, 方向: 卖出}:
                default:
            }
        }
    }
}
```

```
订单控制函数 () {
    初始化现货挂单列表
    for {
        select {
             case 现货订单状态变更信号:
                更新现货挂单列表

                // 合约对冲
                if 现货买入 {
                    以现货成交价格/(1-阈值) * (1-0.8%), 挂合约卖出单(TimeInForce=IOC)
                }else {
                    以现货成交价格/(1+阈值) * (1+0.8%), 挂合约买入单(TimeInForce=IOC)
                }

            case 合约订单状态变更信号:
                if 不是完全成交 {
                    等待1888ms,以市价下单,数量为之前未成交部分
                }

            case message <-现货建议下单channel:
                if message.方向 == 买入 {
                    if 当前USDT仓位 < 预定义下单数量 {
                        break
                    }
                    if 当前没有任何现货挂单 {
                        将挂单数量分成两份, 分别以指定价格和指定价格减去步进值, 向订单列表中添加两单
                    }
                    if 当前挂单为卖出现货 {
                        向订单列表中添加两个撤单并下单的订单(依据当前挂单列表情况,可能一个是撤销并下单,另一个直接下单)
                    }
                    if 当前有现货买入挂单 {
                        if 指定价格 > 现货买入挂单[0].价格 {
                            向订单列表中添加一个撤销现货买入挂单[1], 并以指定价格下单的订单
                        }else if 指定价格 < 现货买入挂单[0].价格 {
                            向订单列表中添加一个撤销现货买入挂单[0], 并以指定价格下单的订单
                        }else {
                            什么都不做
                        }
                    }
                }else {
                    if 当前现货仓位 < 预定义下单数量 {
                        break
                    }
                    if 当前没有任何现货挂单 {
                        将挂单数量分成两份, 分别以指定价格和指定价格加上步进值, 向订单列表中添加两单
                    }
                    if 当前挂单为买入现货 {
                        向订单列表中添加两个撤单并下单的订单(依据当前挂单列表情况,可能一个是撤销并下单,另一个直接下单)
                    }
                    if 当前有现货卖出挂单 {
                        if 指定价格 < 现货卖出挂单[0].价格 {
                            向订单列表中添加一个撤销现货卖出挂单[1], 并以指定价格下单的订单
                        } else if 指定价格 > 现货卖出挂单[0].价格 {
                            向订单列表中添加一个撤销现货卖出挂单[0], 并以指定价格下单的订单
                        } else {
                            什么都不做
                        }
                    }
                }
                订单列表进交易过滤器
                通过websocket下单

            case 终止信号:
                退出
        }
    }
}
```
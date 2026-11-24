# Etapa 6 — Dashboard de métricas

No Windows ou Linux, abra o painel com dados reais de uma campanha da etapa 4:

```text
go run ./cmd/dashboard -metrics results/etapa4-1490099931/metrics.jsonl -emulation results/etapa4-1490099931/emulation.jsonl
```

Substitua o diretório do exemplo pelo caminho de uma campanha local. Os arquivos brutos em `results/` são gerados pela execução da [etapa 4](ETAPA4.md). Abra http://127.0.0.1:8080. Use
`-port` para escolher outra porta.

Para acompanhar novos resultados do servidor, use
`go run ./cmd/dashboard -metrics results/dashboard.jsonl` em um terminal
e adicione `-metrics results/dashboard.jsonl` ao comando `rc2 serve`
em outro. O painel atualiza a cada dois segundos e mostra os últimos 200
registros de cada fonte após serem gravados. A janela exibida pertence à
sessão registrada. O RTT e as retransmissões usam a ponta emissora.
Throughput RC2 inclui os bytes de protocolo observados pela ponta durante
o intervalo da transferência, sem cabeçalhos UDP/IP. Goodput conta bytes úteis.

Perdas são exibidas por direção do emulador a partir dos contadores observados,
com descartes aleatórios e de fila separados. Em transferências reais sem
contadores de perda, o painel informa que essa medição está indisponível.

# Etapa 4 — Experimentos com atraso e perda

A etapa usa emulação em Go com a biblioteca padrão. Não altera as configurações de rede do sistema operacional. A campanha da etapa 3 continua disponível com `go run ./cmd/experiment`, sem `-stage4`.

## Modelo experimental

| Cenário | RTT adicionado | Atraso por sentido | Probabilidade de perda por sentido |
|---|---:|---:|---:|
| S1 | 25 ms | 12,5 ms | 0% |
| S2 | 100 ms | 50 ms | 0% |
| S3 | 100 ms | 50 ms | 5% |
| S4 | 400 ms | 200 ms | 5% |

Cliente e servidor usam sockets UDP reais de loopback, no mesmo processo. Cada direção tem seu próprio gerador pseudoaleatório, fila de 8192 datagramas e um worker que encaminha pacotes no instante de admissão + atraso. Pacotes admitidos em sequência aguardam seus próprios horários; não se aplica um sleep completo serial a cada pacote. A fila preserva a ordem no modelo de atraso constante.

O RTT configurado é **adicionado** ao atraso de loopback, processamento e agendamento. Portanto o RTT medido pode ser maior. O RTT médio do protocolo usa amostras DATA/ACK sem retransmissão (Karn), não o conjunto completo das tentativas. Não registrar o RTT configurado como se fosse medido.

A perda é Bernoulli independente por datagrama, em cada sentido, incluindo autenticação, START, DATA, ACK, FINISH e DONE. Um par DATA/ACK pode falhar se qualquer sentido perder seu datagrama; 5% por sentido não equivale a 5% por troca completa. A taxa observada varia em torno da probabilidade configurada. Retransmissões são contadas separadamente.

Não há limite de banda, jitter configurado ou reprodução de uma topologia real. Os casos S1–S4 são emulações, não medições entre Brasil/EUA ou de um enlace de satélite. Perdas fora do emulador (socket/SO) não são medidas diretamente por ele.

## Validar a integração

```text
go run ./cmd/validate -stage4 -cross
```

Além da suíte normal, ativa `TestStage4UDP`: um upload e um download de 1 MiB em cada cenário, com verificação de hash, relatórios pareados e contadores de perda. Esse teste pequeno **não é um resultado experimental válido para a exigência de 80 MB**. Pode levar alguns minutos.

## Campanha de 80 MiB

Somente depois de a integração passar:

```text
go run ./cmd/experiment -stage4 -repeats 5 -mib 80 -window 64
```

São 40 transferências, 80 registros de transporte e 20 registros do emulador, se tudo terminar. Para executar apenas um cenário, acrescente `-scenario 1` (ou 2, 3, 4). Para variar a janela, repita em execuções separadas com `-window 16` e `-window 256`. Use `-seed` para definir a semente base. O padrão é 2026. O limite de execução de cada par upload/download é duas horas; falhas interrompem a campanha e preservam arquivos parciais de resultados.

Com janela 64, payload de 1172 bytes e RTT de 400 ms, a aproximação janela × payload / RTT dá cerca de 1,5 Mbit/s antes de considerar perdas e outros custos. Um único envio de 80 MiB pode levar vários minutos; a campanha completa pode levar horas. Não use o tempo obtido na etapa 3 para estimar esta campanha.

O arquivo é gerado uma vez, mantido igual entre cenários e verificado no destino após cada operação. RTO adaptativo, buffers solicitados de 4 MiB e janela escolhida são usados nas duas pontas. Ordem dos cenários gira por repetição. Sementes por grupo/repetição estão no manifesto e no registro do emulador; uma semente reproduz a sequência de decisões, mas não garante resultados idênticos se o agendamento alterar a sequência de datagramas/retransmissões.

## Arquivos de saída

Cada execução cria `results/etapa4-<identificador>/`:

- `manifest.json`: ambiente, parâmetros base e sobreposições por cenário, hash e convenção de sementes. Durações da configuração e RTT dos grupos são em nanossegundos.
- `metrics.jsonl` e `metrics.csv`: registros do cliente e servidor por operação/sessão, RTT medido, goodput e retransmissões. `sender_rc2_mbps` usa bytes RC2 oferecidos pelo emissor (incluindo retransmissões e controle do transporte) divididos por sua duração. Exclui comandos de serviço e overhead UDP/IP/Ethernet; não é throughput medido no fio e inclui tentativas descartadas pelo emulador.
- `emulation.jsonl`: um registro por par upload/download e repetição. Contadores por sentido incluem todos os datagramas, comandos de serviço e a espera final. Registram perda sorteada, fila cheia, encaminhamentos, bytes RC2 encaminhados, erro de envio e pendentes/cancelados. Correlacionar pelo grupo/repetição. A média de vazão do emulador inclui o intervalo entre upload e download e não deve ser comparada diretamente com o goodput de uma operação.
- `COMPLETE`: somente após todos os pares, hashes e verificações de fila/erro de envio aprovados.

Fila cheia ou erro de envio invalida a execução para comparação controlada: a campanha termina sem COMPLETE. Datagramas sorteados como perda são esperados, não invalidam a execução. Contadores de pendentes/cancelados são explicitados ao fechar o emulador; podem incluir mensagens redundantes após a conclusão do protocolo e devem ser inspecionados.

## Reproduzir os experimentos

```text
go run ./cmd/validate -large -stage4 -cross
go run ./cmd/experiment -stage4 -repeats 5 -mib 80 -window 16
go run ./cmd/experiment -stage4 -repeats 5 -mib 80 -window 64
go run ./cmd/experiment -stage4 -repeats 5 -mib 80 -window 256
```

Execute as campanhas separadamente, com a mesma máquina e configuração.
Cada execução completa gera 40 transferências verificadas. As tabelas e figuras
da campanha realizada estão na [comparação final](comparacao-etapa4/comparison.md).
Para interpretar a captura de demonstração, consulte [CAPTURA-S3](CAPTURA-S3.md).

## Resultados

Campanhas de 80 MiB com janelas 16, 64 e 256 concluídas e verificadas: 120 transferências, 240 registros de métricas e 60 pares do emulador. Hashes/tamanhos e CSV/JSONL conferidos; zero descartes por fila e erros de envio. Comparação, IC95% e gráficos gerados em [comparacao-etapa4/comparison.md](comparacao-etapa4/comparison.md). Captura S3 e decodificação documentadas em CAPTURA-S3.md e capture-s3/capture.md.

A janela 256 apresentou o maior goodput médio nos quatro cenários e nos dois sentidos. Em upload, o ganho 256/16 foi aproximadamente 15,72 vezes em S1, 15,81 em S2, 7,00 em S3 e 7,14 em S4. Com perdas, o ganho foi menor que a razão entre as janelas (16). Isso é compatível com custos de recuperação e bloqueio da base da janela; não isola causalmente cada custo. Há retransmissões mesmo sem perda injetada: não atribuí-las automaticamente a perda física. Os resultados são de loopback com emulação, não de enlace real entre máquinas.

As campanhas ocorreram sequencialmente e os manifestos não registram hardware/commit; igualdade desses fatores depende do controle da execução pelo operador. Não foi realizado teste de significância entre janelas.

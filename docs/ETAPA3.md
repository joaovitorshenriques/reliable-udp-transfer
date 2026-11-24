# Etapa 3 — Robustez, temporização e observabilidade

Nesta etapa, o servidor passou a atender endpoints simultâneos. O transporte ganhou temporizadores adaptativos, buffers configuráveis e métricas por sessão. A suíte cobre cancelamento, recuperação após falha e transferências concorrentes.

## Temporização e métricas

- RTO adaptativo dos pacotes DATA, com SRTT e variação do RTT; pacotes retransmitidos não fornecem amostras (Karn).
- Cada pacote guarda seu próprio prazo. Novos ACKs não antecipam temporizadores iniciados; após timeout, o intervalo do pacote dobra até o teto.
- Modo fixo por `-adaptive=false` para comparação.
- Buffer de escrita de arquivo de 256 KiB, descarregado antes da sincronização e publicação.
- Buffers UDP solicitados de 4 MiB; `-udp-buffer 0` preserva o padrão do SO. O tamanho efetivo depende do sistema.
- Métricas nos dois lados, correlacionadas por `session`, com `role` igual a `sender` ou `receiver`.
- JSONL por `-metrics` e conversão para CSV.

O estimador usa as equações de suavização da [RFC 6298](https://www.rfc-editor.org/rfc/rfc6298.html), mas não implementa TCP conforme a RFC: RC2 usa temporizadores por pacote, inicial de 500 ms, piso de 50 ms e teto de 2 s para experimentos controlados. O controle continua com `-rto` fixo. A atualização por ACK também difere de amostrar uma vez por RTT.

## Validação

Na raiz do projeto:

```text
go run ./cmd/validate -large -cross
```

O comando inclui a transferência de 80 MiB, análise estática e compilação cruzada. Os resultados estão em [Validação](VALIDACAO.md).

## Campanha A/B/C

```text
go run ./cmd/experiment
```

A campanha usa arquivos de 80 MiB e faz cinco repetições de upload e download por grupo. A ordem dos grupos gira entre ABC, BCA e CAB. A janela e os demais parâmetros são mantidos.

| Grupo | Parâmetros adicionais em ambos os processos |
|---|---|
| A | `-adaptive=false -udp-buffer 0` |
| B | `-adaptive=false -udp-buffer 4194304` |
| C | `-adaptive=true -udp-buffer 4194304` |

A/B isolam buffers; B/C isolam temporização. Todos já usam escrita em blocos, portanto A não reproduz exatamente a etapa 2. A referência antiga foi uma execução de upload com 80,11 s e 1.081 retransmissões; não permite atribuir causalidade.

A campanha roda cliente e servidor em goroutines no mesmo processo, com sockets UDP reais de loopback, sem perdas injetadas. Não mede uma rede entre máquinas nem o custo da CLI em processos separados. O hash do arquivo de destino é relido e verificado após cada operação. As esperas de término do protocolo são preservadas, portanto a execução leva vários minutos mesmo quando cada envio é rápido.

Cada execução cria `results/etapa3-<identificador>/`, sem substituir resultados existentes:

- `manifest.json`: ambiente, tamanho/hash, grupos e configuração (durações de Config em nanossegundos).
- `metrics.jsonl`: métricas completas de cliente e servidor, com grupo e repetição.
- `metrics.csv`: colunas principais para análise; RTT e retransmissões devem ser comparados no emissor.
- `COMPLETE`: criado somente após todas as operações e hashes serem aprovados.

Se ocorrer erro, a campanha para, retorna código diferente de zero e preserva os registros já escritos. Uma operação que falha pode ter apenas o registro do cliente; a ausência de `COMPLETE` identifica campanha incompleta. Os arquivos de dados gerados são temporários e removidos ao encerrar normalmente, cancelar ou retornar erro. Encerramento forçado pode deixar temporários. Reserve aproximadamente 240 MiB livres com a configuração padrão.

Para uma verificação curta, use `go run ./cmd/experiment -repeats 1 -mib 1`.

### Interpretação

- Compare a mesma `session`, nome e operação. No upload o cliente é emissor; no download, o servidor.
- RTT, RTO, SRTT, variação e `adaptive_rto` descrevem o emissor. Zeros no receptor não medem o emissor.
- `data_rto_ms` é a estimativa final para novos dados. `max_packet_rto_ms` inclui o maior intervalo atribuído a um pacote, inclusive backoff.
- `transport_bytes_*` inclui cabeçalho RC2 e payload observados na camada de transferência, com ACKs e retransmissões; exclui UDP/IP/Ethernet e autenticação/listagem. Não é tráfego total no fio.
- Os contadores do receptor incluem espera final; seu `seconds` exclui essa espera. Não dividir esses contadores por `seconds` para calcular throughput no fio.
- Goodput deve ser analisado apenas em transferências bem-sucedidas; falhas podem conter tamanho pretendido e dados parciais.
- Retransmissões não são taxa de perda medida. Descartes de socket exigem captura ou contadores do SO para confirmação.

## Atendimento simultâneo e limites

O servidor despacha datagramas de uma única porta UDP para até **oito endpoints autenticados (IP e porta)**, cada um com estado, prazo de leitura e fila próprios. Cada endpoint executa uma operação por vez; um pedido novo do mesmo endpoint durante uma transferência recebe ocupado. Outros endpoints podem transferir ou listar simultaneamente. Não há multiplexação de vários fluxos dentro de uma única sessão.

Cada fila comporta 256 datagramas; quando cheia, novos datagramas são descartados e o transporte depende de retransmissões. O limite padrão por arquivo é 1 GiB, ajustável com `-max-size`; janela padrão de 64 pacotes. A publicação sem sobrescrita também protege duas tentativas concorrentes de enviar o mesmo nome: no máximo uma publica o destino.

Endpoints ociosos são liberados após aproximadamente dois minutos com a configuração padrão; esse período preserva o cache de pedidos concluídos. Os oito slots incluem endpoints ociosos ainda retidos, portanto várias invocações rápidas da CLI podem atingir o limite. Quando cheio, o servidor informa limite de clientes. O prazo de retenção aumenta se os limites de temporização configurados forem maiores. Não há limite global de armazenamento nem quota de disco por usuário.

# Etapa 3 — temporização e observabilidade

## Revisão atual: atendimento concorrente

O servidor agora usa um leitor UDP e um worker por endpoint autenticado, com limite de oito endpoints, filas de 256 datagramas e deadlines independentes. Cada endpoint continua executando uma operação por vez. O cache de pedidos fica isolado por endpoint; callbacks de métricas são serializados para preservar JSONL. Endpoints ociosos são retidos por cerca de dois minutos (ajustado aos limites de timeout) e depois liberados. Slots incluem os endpoints ainda retidos; fila cheia implica descarte recuperável por retransmissão, sem contador próprio nesta revisão. Não há quota global de disco.

O teste `TestConcurrentClients` usa três clientes com upload/download e um upload parado, testa recusa de senha e limite de clientes. Executa nas variantes memory e udp. A versão memory verifica a lógica sem depender da rede do ambiente; não substitui a execução UDP. O teste de recuperação foi atualizado: outro endpoint pode listar enquanto o upload está incompleto, mas um novo pedido no endpoint ativo recebe ocupado.

O README foi reescrito para comandos comuns `go run` e senha em arquivo via `-password-file`, sem exigir PowerShell. Todos os resultados e descrições de atendimento sequencial abaixo são históricos anteriores a esta revisão.

**Etapa concluída em 12/11/2026.** O usuário executou `go run ./cmd/validate -large -cross`: suíte completa, concorrência UDP real, teste de 80 MiB, vet e compilações Windows/Linux passaram. Consulte `VALIDACAO.md` para números, limites e interpretação das mensagens de encerramento. As seções seguintes preservam o histórico da implementação.

## Robustez após a campanha A/B/C

Implementados ajustes adicionais e aprovados pela suíte UDP no terminal local do usuário em 12/11/2026:

- Pedidos novos durante uma transferência recebem resposta de servidor ocupado, inclusive quando vêm da mesma porta do cliente ativo. A retransmissão do pedido original continua recebendo a resposta original.
- Esperas de controle consultam cancelamento a cada intervalo de até 100 ms, preservando o prazo do RTO e a contagem de retransmissões. Isso limita a espera de leitura; não é uma garantia de tempo real para disco/agendamento.
- O timeout de inatividade do receptor é renovado ao aceitar START inicial ou dados novos. Repetir START ou DATA já recebido não mantém indefinidamente um upload abandonado. Continua não existindo um limite total de duração para um cliente que fornece dados novos.

O teste `TestInterruptedUploadRecovery` inicia um upload parcial, verifica ocupado para outro cliente e para novo pedido do cliente ativo, provoca inatividade com replays ou cancela o servidor, verifica limpeza do temporário e, no caso de inatividade, faz novo upload/listagem para demonstrar recuperação. Os testes `TestControlCancellationWithLongRTO` e `TestControlPollingPreservesRTO` verificam cancelamento e preservação do timeout sem depender de sockets do ambiente.

Execute no PowerShell local: `go run ./cmd/validate -cross`. Não é necessário repetir a campanha A/B/C para validar esses casos de falha. O servidor permanece sequencial; estes ajustes não implementam sessões simultâneas.

## Continuação: campanha local automatizada

Por decisão do usuário, não é necessário executar testes em Linux nesta etapa. A compatibilidade é mantida pelo código Go multiplataforma e pela compilação cruzada; isso não equivale a uma execução validada naquele SO.

No Windows, na raiz do repositório:

```powershell
go run ./cmd/experiment
```

O mesmo comando funciona no Linux com Go instalado, sem PowerShell. O padrão é um arquivo de 80 MiB, cinco repetições por grupo e upload/download em cada repetição: 30 transferências e 60 registros de métricas quando tudo termina. Os grupos são A (fixo/buffers do SO), B (fixo/4 MiB) e C (adaptativo/4 MiB). Janela e demais parâmetros permanecem constantes. A ordem gira entre ABC, BCA e CAB para reduzir associação com o momento da execução; isso não elimina todos os efeitos de ordem/cache.

A campanha roda cliente e servidor em goroutines no mesmo processo, com sockets UDP reais de loopback, sem perdas injetadas. Não mede uma rede entre máquinas nem o custo da CLI em processos separados. O hash do arquivo de destino é relido e verificado após cada operação. As esperas de término do protocolo são preservadas, portanto a execução leva vários minutos mesmo quando cada envio é rápido.

Cada execução cria `results/etapa3-<identificador>/`, sem substituir resultados existentes:

- `manifest.json`: ambiente, tamanho/hash, grupos e configuração (durações de Config em nanossegundos).
- `metrics.jsonl`: métricas completas de cliente e servidor, com grupo e repetição.
- `metrics.csv`: colunas principais para análise; RTT e retransmissões devem ser comparados no emissor.
- `COMPLETE`: criado somente após todas as operações e hashes serem aprovados.

Se ocorrer erro, a campanha para, retorna código diferente de zero e preserva os registros já escritos. Uma operação que falha pode ter apenas o registro do cliente; a ausência de `COMPLETE` identifica campanha incompleta. Os arquivos de dados gerados são temporários e removidos ao encerrar normalmente, cancelar ou retornar erro. Encerramento forçado pode deixar temporários. Reserve aproximadamente 240 MiB livres com a configuração padrão.

Para um teste curto do executor: `go run ./cmd/experiment -repeats 1 -mib 1`. Esse teste não substitui a campanha de 80 MiB. Para os resultados finais desta comparação, use os valores padrão e envie `manifest.json`, `metrics.jsonl` e `metrics.csv` para análise. Não interprete uma execução incompleta como benchmark aprovado.

**Estado: validação funcional aprovada no Windows.** A suíte completa, incluindo transferência de 80 MiB, análise estática e compilação Windows/Linux passou no terminal local do usuário. A comparação controlada de desempenho permanece pendente; Linux tem compilação verificada, sem execução nativa no escopo atual. Veja os resultados atuais em `VALIDACAO.md`; os bloqueios abaixo são registros históricos do ambiente anterior.

## Mudanças

- RTO adaptativo dos pacotes DATA, com SRTT e variação do RTT; pacotes retransmitidos não fornecem amostras (Karn).
- Cada pacote guarda seu próprio prazo. Novos ACKs não antecipam temporizadores iniciados; após timeout, o intervalo do pacote dobra até o teto.
- Modo fixo por `-adaptive=false` para comparação.
- Buffer de escrita de arquivo de 256 KiB, descarregado antes da sincronização e publicação.
- Buffers UDP solicitados de 4 MiB; `-udp-buffer 0` preserva o padrão do SO. O tamanho efetivo depende do sistema.
- Métricas nos dois lados, correlacionadas por `session`, com `role` igual a `sender` ou `receiver`.
- JSONL por `-metrics` e conversão para CSV.

O estimador usa as equações de suavização da [RFC 6298](https://www.rfc-editor.org/rfc/rfc6298.html), mas não implementa TCP conforme a RFC: RC2 usa temporizadores por pacote, inicial de 500 ms, piso de 50 ms e teto de 2 s para experimentos controlados. O controle continua com `-rto` fixo. A atualização por ACK também difere de amostrar uma vez por RTT.

## Diagnóstico do bloqueio

O erro de Controle de Aplicativo ocorre antes da execução. Não representa falha de uma asserção, mas impede validar o código. Segundo a [Microsoft](https://learn.microsoft.com/en-us/windows/security/application-security/application-control/app-control-for-business/operations/appcontrol-debugging-and-troubleshooting), os eventos CodeIntegrity identificam a política responsável.

Em PowerShell **como administrador**, na raiz do projeto:

```powershell
.\scripts\diagnose-app-control.ps1
```

O script só lê eventos 3077 do projeto. Nenhuma proteção é alterada. A solução depende da política identificada: autorização de desenvolvimento pelo administrador responsável ou ambiente de testes apropriado. Nenhuma dependência nova foi instalada.

## Validação após resolver o ambiente

Comando comum a Windows e Linux, na raiz do repositório:

```text
go run ./cmd/validate -large -cross
```

Ele executa testes (incluindo 80 MiB), análise estática e compilação nativa, parando no primeiro erro. Também verifica compilação para o outro SO; isso não atesta execução naquele sistema, que está fora do escopo atual. Com compilador portátil, substitua `go` por `.\.tools\go\bin\go.exe` no Windows ou por `sh scripts/go.sh` no Linux. Não é preciso instalar PowerShell no Linux.

O teste grande é uma verificação de integridade do transporte, não substitui a comparação de upload/download com cinco repetições por grupo abaixo.

```powershell
.\scripts\go.ps1 test ./... -count=1 -v
.\scripts\go.ps1 vet ./...
# Encerre o servidor antigo antes de substituir o executável.
.\scripts\go.ps1 build -o bin/rc2.exe ./cmd/rc2
```

Critérios: suíte aprovada, incluindo RTO, recuperação de perdas nos modos fixo/adaptativo e correlação das métricas entre as pontas. Não avançar para benchmarks enquanto esses testes estiverem bloqueados ou falhando.

## Avaliação manual com métricas

Após aprovação da suíte, terminal do servidor:

```powershell
$env:RC2_PASSWORD = 'senha-de-teste'
.\bin\rc2.exe serve -addr 127.0.0.1:9000 -dir data/server -metrics results/server.jsonl
```

Terminal do cliente, usando o arquivo de 80 MiB:

```powershell
$env:RC2_PASSWORD = 'senha-de-teste'
.\bin\rc2.exe upload -addr 127.0.0.1:9000 -file work/original.bin -name etapa3.bin -metrics results/client.jsonl
Start-Sleep -Seconds 8
.\bin\rc2.exe download -addr 127.0.0.1:9000 -name etapa3.bin -file work/etapa3.bin -metrics results/client.jsonl
Get-FileHash work/original.bin, work/etapa3.bin -Algorithm SHA256
.\scripts\export-metrics.ps1 -InputPaths results/client.jsonl,results/server.jsonl -OutputPath results/comparacao.csv
```

Use nomes novos em cada repetição. O servidor registra o upload após a espera final do receptor. JSONL recebe um objeto por linha, em append. Falhas anteriores ao transporte (por exemplo, autenticação rejeitada) não geram registro de transferência no servidor. A senha não é incluída nos registros.

### Interpretação

- Compare a mesma `session`, nome e operação. No upload o cliente é emissor; no download, o servidor.
- RTT, RTO, SRTT, variação e `adaptive_rto` descrevem o emissor. Zeros no receptor não medem o emissor.
- `data_rto_ms` é a estimativa final para novos dados. `max_packet_rto_ms` inclui o maior intervalo atribuído a um pacote, inclusive backoff.
- `transport_bytes_*` inclui cabeçalho RC2 e payload observados na camada de transferência, com ACKs e retransmissões; exclui UDP/IP/Ethernet e autenticação/listagem. Não é tráfego total no fio.
- Os contadores do receptor incluem espera final; seu `seconds` exclui essa espera. Não dividir esses contadores por `seconds` para calcular throughput no fio.
- Goodput deve ser analisado apenas em transferências bem-sucedidas; falhas podem conter tamanho pretendido e dados parciais.
- Retransmissões não são taxa de perda medida. Descartes de socket exigem captura ou contadores do SO para confirmação.

## Comparação controlada

Mantenha arquivo e janela; faça pelo menos cinco repetições por grupo e registre os parâmetros. Reinicie o servidor com os mesmos parâmetros de cada grupo:

| Grupo | Parâmetros adicionais em ambos os processos |
|---|---|
| A | `-adaptive=false -udp-buffer 0` |
| B | `-adaptive=false -udp-buffer 4194304` |
| C | `-adaptive=true -udp-buffer 4194304` |

A/B isolam buffers; B/C isolam temporização. Todos já usam escrita em blocos, portanto A não reproduz exatamente a etapa 2. A referência antiga foi uma execução de upload com 80,11 s e 1.081 retransmissões; não permite atribuir causalidade.

Depois da validação: variar janela (16/64/256), testar entre máquinas e construir os quatro cenários controlados. Sessões simultâneas, throughput no fio e perda experimental permanecem pendentes.

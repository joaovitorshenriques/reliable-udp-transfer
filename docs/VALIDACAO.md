# Registro de validação — 10/11/2026

Ambiente: Windows amd64, Go 1.27.4 portátil, rede loopback, execução pelo ambiente do Codex. Dependências externas de Go: nenhuma. Estes resultados são verificações de engenharia da etapa 1, não os quatro cenários exigidos no relatório.

## Executado

| Verificação | Resultado |
|---|---|
| `go test ./... -count=1 -v` | Suíte funcional passou; teste grande é opt-in |
| `go vet ./...` | Passou, sem diagnósticos |
| `go build -o bin/rc2.exe ./cmd/rc2` | Executável gerado |
| CLI em dois processos UDP, arquivo de 15 bytes | Sucesso no emissor/receptor, hashes iguais |
| `RC2_LARGE_TEST=1 go test ./internal/transfer -run TestLargeFile -v -count=1 -timeout 3m` | Passou com 83.886.080 bytes |

## Transferência de 80 MiB

- Bytes recebidos e conferidos: **83.886.080**.
- SHA-256: `42d12a7bb4fe5161f36dd20908f31ea01e2ae86741f32adc970dc527f22ad4f4`.
- Janela configurada: 64 pacotes.
- RTO fixo: 500 ms.
- Tempo medido no emissor: 46,304 s.
- Goodput do emissor: 14,493 Mbit/s.
- Retransmissões de dados: 636.
- Duração total do teste: 53,55 s, incluindo criação do arquivo e espera final do receptor.

Não foi injetada perda nesse teste grande. Ainda assim houve retransmissões; **não atribuir automaticamente isso à perda no enlace**. Descarte em buffers locais e atrasos de processamento/agendamento são hipóteses a investigar com captura e instrumentação. É uma amostra única neste ambiente, sem significado de benchmark comparativo. A próxima etapa de desempenho deverá investigar buffers de socket, RTO e custo por pacote.

## Correção encontrada durante os testes

A tentativa inicial de publicar arquivos por hard link falhou neste ambiente Windows. A implementação final usa `MoveFile` no Windows, que não substitui destino existente; nos outros sistemas mantém hard link. Os testes de transferência foram executados novamente após a correção. Há teste específico para garantir que a publicação recusa um arquivo criado antes dela.

## Ainda não validado

- Linux/macOS e transferência entre máquinas reais.
- Cenários com RTT 25/100/400 ms e perda controlada de 5%.
- Variação de janela e análise estatística.
- Capturas Wireshark, race detector e campanha prolongada de fuzzing (somente sementes do fuzz foram executadas).
- Autenticação, listagem, serviço de upload/download, mini-CDN, dashboard e nuvem: ainda não implementados.

## Reproduzir

Siga os comandos do README. Os testes comuns criam e removem seus próprios arquivos temporários. O teste de 80 MiB demanda espaço temporário para duas cópias do arquivo. Os arquivos de smoke test da CLI estão em `work/` e não devem ser entregues como código-fonte.

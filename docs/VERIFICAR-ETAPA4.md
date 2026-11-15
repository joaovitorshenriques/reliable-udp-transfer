## Consolidação experimental — 14/11/2026

Campanhas de 80 MiB com janelas 16, 64 e 256 concluídas e verificadas: 120 transferências, 240 registros de métricas e 60 pares do emulador. Hashes/tamanhos e CSV/JSONL conferidos; zero descartes por fila e erros de envio. Comparação, IC95% e gráficos gerados em [comparacao-etapa4/comparison.md](comparacao-etapa4/comparison.md). Captura S3 e decodificação documentadas em CAPTURA-S3.md e capture-s3/capture.md.

A janela 256 apresentou o maior goodput médio nos quatro cenários e nos dois sentidos. Em upload, o ganho 256/16 foi aproximadamente 15,72 vezes em S1, 15,81 em S2, 7,00 em S3 e 7,14 em S4. Com perdas, o ganho foi menor que a razão entre as janelas (16). Isso é compatível com custos de recuperação e bloqueio da base da janela; não isola causalmente cada custo. Há retransmissões mesmo sem perda injetada: não atribuí-las automaticamente a perda física. Os resultados são de loopback com emulação, não de enlace real entre máquinas.

As campanhas ocorreram sequencialmente e os manifestos não registram hardware/commit; igualdade desses fatores depende do controle da execução pelo operador. Não foi realizado teste de significância entre janelas. A validação completa combinada `go run ./cmd/validate -large -stage4 -cross` foi aprovada no Windows em 14/11/2026, com build Linux aprovado (sem execução Linux). Não repetir campanhas longas já aprovadas.

# Verificação final da etapa 4

A implementação e as ferramentas de análise estão disponíveis. O fechamento experimental foi aprovado; os comandos abaixo ficam como roteiro de reprodução, sem necessidade de repetir as campanhas já verificadas.

## 1. Suíte completa (Windows ou Linux)

```text
go run ./cmd/validate -large -stage4 -cross
```

Todos os testes habilitados, vet e builds devem passar. No Windows, o build Linux é somente compilação. Mensagens de cancelamento nos testes que provocam interrupções são esperadas quando o teste termina em PASS.

## 2. Campanhas restantes

Execute sequencialmente, na mesma máquina e sem alterar código/configuração. Preserve a campanha existente de janela 64. Cada comando cria sua própria pasta e pode levar horas, especialmente a janela 16. Não suspender o computador.

```text
go run ./cmd/experiment -stage4 -repeats 5 -mib 80 -window 16
go run ./cmd/experiment -stage4 -repeats 5 -mib 80 -window 256
```

Cada campanha deve terminar com COMPLETE, 40 transferências com hash correto, 80 métricas e 20 registros do emulador. Não misturar execuções incompletas. Janela 1 é referência opcional Stop-and-Wait, não faz parte desta matriz de três janelas.

## 3. Comparação

Substitua PASTA_W16 e PASTA_W256 pelos caminhos impressos pelos comandos anteriores:

```text
python scripts/compare-stage4.py PASTA_W16 results/etapa4-1490099931 PASTA_W256 --out docs/comparacao-etapa4
```

Em Linux, use python3 se necessário. Requer Matplotlib, instalado localmente em .tools/python-packages neste ambiente. O script exige cinco repetições, quatro cenários, tamanho >=80 MiB, mesmo hash/configuração/ambiente registrado e janelas distintas 16/64/256. Produz CSV, relatório e gráficos PNG/PDF com IC95%. As verificações do manifesto não comprovam igualdade de hardware ou revisão do código: confirme isso ao executar.

## 4. Captura já realizada

```text
python scripts/inspect-capture.py results/captures/s3-w64.pcapng --out docs/capture-s3
```

Usa TShark do PATH ou caminho padrão Windows; `--tshark CAMINHO` permite outro local. Resultado conferido: 3.773 pacotes RC2 válidos e 143 DATA repetidos visíveis. Veja [inspeção](capture-s3/capture.md) e [contadores do experimento](CAPTURA-S3.md). O CSV associa frames, sessões, sequências e primeira ocorrência de DATA. Não exporta payload de autenticação.

Para abrir o Wireshark com o decodificador no PowerShell:

```powershell
& "C:/Program Files/Wireshark/Wireshark.exe" -X lua_script:scripts/rc2.lua -r results/captures/s3-w64.pcapng
```

No Linux, substitua o executável por `wireshark`. Filtros de exibição: `rc2`, `rc2.kind == 2` (DATA), `rc2.kind == 3` (ACK). Escolha um par de frames do relatório e filtre por `frame.number == N || frame.number == M`. Para acompanhar ACKs, filtre `rc2.session == 0xID && rc2.seq == NUMERO`, usando ID e sequência do relatório. O Lua mostra campos; a conferência CRC é feita pelo analisador Python.

Capturas de loopback observam somente datagramas encaminhados; o RTT virtual inclui espera anterior ao envio. Não interpretar ausência no PCAP como prova isolada de perda de rede. Esta captura de 1 MiB documenta funcionamento, não desempenho de 80 MiB.

## Critério de encerramento

Suíte completa aprovada; três campanhas completas e comparáveis; gráficos/CSV gerados; captura e exemplos DATA/ACK documentados. As três campanhas foram analisadas e a validação completa foi aprovada: etapa 4 encerrada neste escopo. Mini-CDN pertence à etapa 5.

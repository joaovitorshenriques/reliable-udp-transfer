# Etapa 5 — Mini-CDN

O comando `rc2 cdn` consulta de duas a oito réplicas, alterna a ordem a cada rodada e escolhe o menor RTT mediano entre servidores com maioria de respostas válidas. Padrão: cinco sondagens autenticadas, sem retransmissão, timeout de um segundo. Falhas são registradas e não viram amostras de RTT zero. Empates preservam a ordem de entrada. O RTT inclui processamento da aplicação. Cada sondagem usa sessão aleatória; respostas precisam ter origem e sessão corretas e CRC válido.

O servidor aceita a nova operação `probe` dentro de REQUEST/RESPONSE, mantendo o formato RC2U versão 1. Comandos existentes permanecem compatíveis. Réplicas antigas rejeitam probe e não são selecionadas. Um socket por réplica é reutilizado para não consumir um novo endpoint a cada amostra. A seleção e as métricas de download são impressas como JSON. `-selection` grava um arquivo novo, sem sobrescrever outro.

## Preparação em duas máquinas

Use a mesma versão do projeto, senha e cópia idêntica do arquivo em ambos os servidores. A distribuição do arquivo é manual nesta versão. Verifique previamente o SHA-256 das réplicas; a transferência verifica o hash anunciado pelo servidor, mas não certifica que réplicas diferentes contêm a mesma versão. Não há fallback automático após erro de download.

Em cada máquina, crie `password.txt` com a mesma senha e coloque o arquivo em `data/server/arquivo.bin`. Execute:

```text
go run ./cmd/rc2 serve -addr 0.0.0.0:9000 -dir data/server -password-file password.txt
```

Use uma rede de teste e permita UDP 9000 no firewall somente para o cliente. Nenhuma regra é alterada automaticamente. O cliente pode estar em uma das máquinas, mas registre essa assimetria na análise. Para seleção entre caminhos reais, prefira cliente separado dos dois servidores.

No cliente, substitua IP_A e IP_B pelos endereços reais:

```text
go run ./cmd/rc2 cdn -servers IP_A:9000,IP_B:9000 -password-file password.txt -samples 5 -probe-only -selection selecao-sondagem.json
go run ./cmd/rc2 cdn -servers IP_A:9000,IP_B:9000 -password-file password.txt -samples 5 -name arquivo.bin -file recebido-cdn.bin -selection selecao-download.json
```

São comandos iguais no Windows/Linux. Os destinos devem ser novos e suas pastas precisam existir. Use `-window 64` para fixar a janela desejada. O arquivo selecionado deve ser >=80 MiB para a avaliação de desempenho.

## Verificação

```text
go run ./cmd/validate -cross
```

Inclui TestProbeAuthenticationAndReuse (senha incorreta rejeitada, cinco probes e listagem posterior no mesmo socket) e testes determinísticos de seleção (outlier, indisponibilidade, cancelamento, duplicatas).

Para um ensaio funcional em uma máquina, suba dois servidores com portas 9000/9001 e diretórios separados. Use `127.0.0.1:9000,127.0.0.1:9001` no cliente. Isso valida o fluxo, mas não conclui a exigência de duas máquinas.

Teste também um servidor indisponível, senha incorreta em ambos e destino existente. Não deve haver download quando nenhuma réplica for elegível. Se o servidor escolhido falhar durante o download, o comando retorna erro; repetir exige nova seleção e destino válido.

## Como reproduzir a avaliação

1. Registrar IPs, máquinas, sistema, versão Go, hash do arquivo e configuração.
2. Capturar na interface da rede real com filtro `udp port 9000` (loopback só para ensaio local). Carregar scripts/rc2.lua: sondagens aparecem como REQUEST/RESPONSE; a sessão permite correlacionar pares. Não compartilhar capturas com senha real: a autenticação atual não é criptografada.
3. Fazer cinco repetições de seleção+download e cinco downloads diretos de cada réplica, alternando a ordem. Usar nomes locais/relatórios novos em cada repetição. Comparar RTT das sondagens, goodput, retransmissões e integridade. Menor RTT não garante maior goodput.
4. Conferir que o endereço selecionado corresponde à menor mediana elegível em cada JSON e que o tráfego DATA vem desse servidor.

Campanha e captura concluídas; consultar o encerramento acima.

## Resultados em rede local

Desktop 192.168.2.128 (cabo, cliente e réplica) e notebook 192.168.2.150 (Wi-Fi). Foram verificadas 15 transferências de 80 MiB com relatórios pareados e hashes iguais. Cinco seleções escolheram corretamente o desktop. O PCAP confirma 50 sondagens respondidas e START, primeiro DATA e DONE das 15 sessões. CRC validado nos frames exportados, não em todos os pacotes da captura.

Resultados: [análise](results-etapa5/analysis.md) e [evidências da captura](results-etapa5/capture.md). Limitações: cliente local favorece o desktop; rodada 5 retomada após limite de endpoints; não foi uma campanha contínua nem comparação entre dois caminhos remotos.

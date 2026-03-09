     oooooooooooo  oooooo      oooooo  ooo        ooooo
     `888'     `8   `YYY'       `V'    `88.       .888' 
      888             YYY       V       888b     d'888
      888ooooo8        YYY     V        8 Y88. .P  888
      888               YYY   V         8  `888'   888
      888      ,8        YYYxV          8    Y     888
     o888booooooo        o888o         o8o        o888o

ooooooooooo.   ooo        ooooo  ooooooooooo.     ,ooooooo.
`888'    `Y8b  `88.       .888'  `888'    `Y8b  ,d88"    `8b
 888     d88'   888b     d'888    888     d88'  888
 888bood88'     8 Y88. .P  888    888bood88'    888
 888            8  `888'   888    888           888
 888            8    Y     888    888           'b88.    ,o9
o888o          o8o        o888o  o888o            `8888888P

----------------------------------------------------------------

Node for running EVM-PMPC

----------------------------------------------------------------

There are bootstrap nodes ran by the team, which can be used to 
bootstrap the network. And then there are nodes to be ran by the 
users which allows you to perform MPC operations.

Run the following command to build the node:

make build

To deploy it:

make docker-node
make docker-bootstrap

To run a node:
make run-worker

To run a bootstrap node:
make run-bootstrap

-----
TODO:
-----
- when a bootstrap node starts, it sends its peerID to 'api'.

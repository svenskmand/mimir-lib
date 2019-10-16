# Mimir-Lib

Mimir-Lib is a stand-alone library to decide on which groups (phisical-hosts, virtual-hosts, zones, etc.) to place entities (databases, containers, processes, etc.). Mimir-Lib evaluates a customizable predicate for each group to determine if the group is useable or not. Then Mimir-Lib orders all the useable groups according to an ordering where the best groups are ordered first. The best group is then picked for the given entity.

## Resources

### Blogs

- [Percona Live 2017 Blog Post](https://www.percona.com/blog/2017/04/20/percona-live-featured-session-casper-kejlberg-rasmussen-placing-databases-uber/)

### Tech Talks

- [Percona Live 2017, Santa Clara][Video](https://www.youtube.com/watch?v=dd3k3J4k7OQ&t=2s)

## License
Mimir-Lib is a fork of Uber Peloton which is under the Apache 2.0 license so Mimir-Lib is using the same license. See the files APACHE-LICENSE and LICENSE for details.

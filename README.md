# mlik

This is a companion to [snid](https://github.com/AGWA/sni). While snid can proxy TLS connections
to IPv6 backends this service can proxy requests for 
[HTTP-01 ACME challenges](https://letsencrypt.org/docs/challenge-types/) towards an IPv6 upstream
and return a https redirect for everything else.
This service is created with minimal dependencies.
#ifndef WAFIELIB_LIBRARY_H
#define WAFIELIB_LIBRARY_H
#include <modsecurity/transaction.h>

typedef struct {
    const unsigned char *key;
    const unsigned char *value;
} EvaluationRequestHeader;

typedef struct {
    char *client_ip;
    char *uri;
    char *http_method;
    char *http_version;
    char *body;
    size_t headers_count;
    EvaluationRequestHeader *headers;
    Transaction *transaction;
} EvaluationRequest;

void wafie_library_init(char const *config_path);

int wafie_process_request_headers(EvaluationRequest const *request);

int wafie_process_request_body(EvaluationRequest const *request);

void wafie_init_request_transaction(EvaluationRequest *request);

void wafie_transaction_cleanup(EvaluationRequest const *request);

void wafie_dump_rules();

void wafie_cleanup(char const *error, RulesSet *rules, ModSecurity *modsec);

int wafie_add_rule(char const *rule);



#endif //WAFIELIB_LIBRARY_H

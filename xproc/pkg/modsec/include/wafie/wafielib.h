#ifndef WAFIELIB_LIBRARY_H
#define WAFIELIB_LIBRARY_H
#include <stdint.h>
#include <modsecurity/transaction.h>

typedef struct {
    const unsigned char *key;
    const unsigned char *value;
} WafieEvaluationRequestHeader;

typedef struct {
    char *config_path;
    uint32_t protection_id;
} WafieRuleSetConfig;

typedef struct {
    uint32_t protection_id;
    RulesSet *rules;
} WafieRuleSet;

typedef struct {
    char *client_ip;
    char *uri;
    char *http_method;
    char *http_version;
    char *body;
    size_t headers_count;
    int total_loaded_rules;
    uint32_t protection_id;
    WafieEvaluationRequestHeader *headers;
    Transaction *transaction;
} WafieEvaluationRequest;

void wafie_init();

void wafie_init_transaction(WafieEvaluationRequest *request);

void wafie_cleanup(WafieEvaluationRequest const *request);

void wafie_load_rule_sets(WafieRuleSetConfig cfg[], int cfg_size);

int wafie_process_request_headers(WafieEvaluationRequest const *request);

int wafie_process_request_body(WafieEvaluationRequest const *request);

#endif //WAFIELIB_LIBRARY_H

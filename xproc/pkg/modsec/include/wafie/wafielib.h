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
    char *protocol;
    char *request_body;
    char *response_body;
    char *intervention_url;
    uint32_t response_code;
    size_t request_headers_count;
    size_t response_headers_count;
    int total_loaded_rules;
    uint32_t protection_id;
    WafieEvaluationRequestHeader *request_headers;
    WafieEvaluationRequestHeader *response_headers;
    Transaction *transaction;
} WafieEvaluationRequest;

void wafie_init();

void wafie_init_transaction(WafieEvaluationRequest *request);

void wafie_cleanup(WafieEvaluationRequest *request);

void wafie_load_rule_sets(WafieRuleSetConfig cfg[], int cfg_size);

int wafie_process_request_headers(WafieEvaluationRequest *request);

int wafie_process_request_body(WafieEvaluationRequest *request);

int wafie_process_response_headers(WafieEvaluationRequest *request);

int wafie_process_response_body(WafieEvaluationRequest *request);

#endif //WAFIELIB_LIBRARY_H

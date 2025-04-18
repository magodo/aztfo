#!/bin/bash

usage() {
    cat << EOF
Usage: ./${MYNAME} [options] <file>

Options:
    -h|--help           Show this message
    --wsp               The path to the TF workspace (with the provider correctly setup) 

Arguments:
    file                The output file generated by aztfo
EOF
}


die() {
    echo -e "$@" >&2
    exit 1
}

main() {
    wsp_dir="."
    while :; do
        case $1 in
            -h|--help)
                usage  
                exit 1
                ;;
            --wsp)
                shift
                wsp_dir=$1
                ;;
            --)
                shift
                break
                ;;
            *)
                break
                ;;
        esac
        shift
    done

    local expect_n_arg
    expect_n_arg=1
    [[ $# = "$expect_n_arg" ]] || die "wrong arguments (expected: $expect_n_arg, got: $#)"

    file=$1

    command -v terraform > /dev/null || die '"terraform" not installed'
    command -v jq > /dev/null || die '"jq" not installed'

    ds_diff="$(diff <(terraform -chdir=$wsp_dir providers schema -json | jq '.provider_schemas."registry.terraform.io/hashicorp/azurerm".data_source_schemas | keys | .[]') <(jq '[.[] | select(.id.is_data_source == true)] | .[].id.name' < $file))"
    if [[ -n $ds_diff ]]; then
        die "Diff data sources (expect vs actual):\n$ds_diff"
    fi

    res_diff="$(diff <(terraform -chdir=$wsp_dir providers schema -json | jq '.provider_schemas."registry.terraform.io/hashicorp/azurerm".resource_schemas | keys | .[]') <(jq '[.[] | select(.id.is_data_source == false)] | .[].id.name' < $file))"
    if [[ -n $res_diff ]]; then
        die "Diff resources (expect vs actual):\n$res_diff"

    fi
}

main "$@"

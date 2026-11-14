# nzlauncher completion for bash. Network names come from the running system
# through `nzlauncher wifi list`; everything else is static.

_nzlauncher() {
    local cur=${COMP_WORDS[COMP_CWORD]} cword=$COMP_CWORD
    local -a words=("${COMP_WORDS[@]}")
    local commands="bar tui gtk pick dmenu query serve setup wifi tweak catalog focus ws help man completion"

    if (( cword == 1 )); then
        mapfile -t COMPREPLY < <(compgen -W "$commands -h --help -V --version" -- "$cur")
        return
    fi

    local candidates=""
    case ${words[1]} in
        wifi)
            if (( cword == 2 )); then
                candidates="connect join disconnect list"
            elif (( cword == 3 )) && [[ ${words[2]} == connect || ${words[2]} == join ]]; then
                # names can hold spaces: one per line, quoted on the way in
                local name
                COMPREPLY=()
                while IFS= read -r name; do
                    [[ $name == "$cur"* ]] && COMPREPLY+=("$(printf '%q' "$name")")
                done < <(nzlauncher wifi list 2>/dev/null)
                return
            fi ;;
        tweak)
            if (( cword == 2 )); then
                candidates="power dnd nightlight awake normal"
            elif (( cword == 3 )); then
                case ${words[2]} in
                    power) candidates="power-saver balanced performance" ;;
                    dnd) candidates="on off" ;;
                    nightlight) candidates="off 3500 4000 4500 5000" ;;
                    awake) candidates="screen sleep all on off" ;;
                esac
            elif (( cword == 4 )) && [[ ${words[2]} == awake ]]; then
                candidates="on off 1800 3600 7200"
            fi ;;
        catalog) (( cword == 2 )) && candidates="status encrypt decrypt edit" ;;
        ws) candidates="move 1 2 3 4 5 6 7 8 9" ;;
        pick) candidates="-p -i" ;;
        help) (( cword == 2 )) && candidates=$commands ;;
        completion) (( cword == 2 )) && candidates="zsh bash fish" ;;
    esac
    mapfile -t COMPREPLY < <(compgen -W "$candidates" -- "$cur")
}

complete -F _nzlauncher nzlauncher

import { Box, FormControl, Input, LinearProgress } from '@mui/joy';
import * as React from 'react';
import { useTimer } from 'use-timer';


const normalize = (value: number, ws: number) => (value * 100 / ws);

interface TokenEnterProps {
    placeholder: string
    userid: string
    value: string
    waitSecs: number
    tokenCreated: Date

    onTokenChange: (s: string) => void
    onTokenSubmit: () => void
    onTimeout: () => void
    onNoUser: () => void
}


export const TokenEnter = ({ placeholder, userid, value, waitSecs, tokenCreated, onTokenChange, onTokenSubmit, onTimeout, onNoUser }: TokenEnterProps) => {
    const { time, start, pause, reset } = useTimer({
        initialTime: 0,
        endTime: waitSecs,
        timerType: 'INCREMENTAL',
        onTimeOver: () => {
            onTimeout();
        },
    });

    const checkEnter = (evt) => {
        if (evt.key === "Enter") {
            onTokenSubmit();
            return false;
        }
        return true;
    }

    React.useEffect(() => {
        start();
    }, []);

    if (!userid) onNoUser();

    const val = normalize(time, waitSecs);

    return (
        <Box >
            <LinearProgress sx={{ m: 1 }} determinate variant="plain" value={val} />
            <FormControl>
                <Input
                    placeholder={placeholder}
                    autoFocus
                    value={value}
                    onKeyDown={(evt) => checkEnter(evt)}
                    onChange={(evt) => onTokenChange(evt.target.value)} />
            </FormControl>
        </Box>
    );
}
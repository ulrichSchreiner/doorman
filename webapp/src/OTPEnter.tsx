import { Box, FormControl, Input } from '@mui/joy';
import * as React from 'react';

interface OTPEnterProps {
    placeholder: string
    userid: string
    value: string
    onTokenChange: (s: string) => void
    onTokenSubmit: () => void
    onNoUser: () => void
}
export const OTPEnter = ({ placeholder, userid, value, onTokenChange, onTokenSubmit, onNoUser }: OTPEnterProps) => {

    const checkEnter = (evt: React.KeyboardEvent) => {
        if (evt.key === "Enter") {
            onTokenSubmit();
            return false;
        }
        return true;
    }

    if (!userid) onNoUser();

    return (
        <Box>
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
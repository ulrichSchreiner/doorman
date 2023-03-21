import { Box, FormControl, Input } from '@mui/joy';
import * as React from 'react';

interface UserProps {
    placeholder: string
    value: string
    onUserChange: (s: string) => void
    onUserSubmit: () => void
}

export const User = ({ placeholder, value, onUserChange, onUserSubmit }: UserProps) => {

    const checkEnter = (evt: React.KeyboardEvent) => {
        if (evt.key === "Enter") {
            onUserSubmit();
            return false;
        }
        return true;
    }

    return (
        <Box ><FormControl>
            <Input
                placeholder={placeholder}
                autoFocus
                value={value}
                onKeyDown={(evt) => checkEnter(evt)}
                onChange={(evt) => onUserChange(evt.target.value)} />
        </FormControl>
        </Box>
    );
}
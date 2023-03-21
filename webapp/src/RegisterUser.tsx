import { Box } from '@mui/joy';
import * as React from 'react';

interface RegisterUserProps {
    userid: string
    onNoUser: () => void
}
export const RegisterUser = ({ userid, onNoUser }: RegisterUserProps) => {

    if (!userid) onNoUser();

    return (
        <Box sx={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: "space-around",
            textAlign: "justify",
            fontFamily: 'Roboto',
        }}>
            You are not a registerd user. Please click 'Register' to receive an EMail with a registration link.
        </Box>
    );
}
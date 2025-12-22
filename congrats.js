// ROT13 decoder script
function rot13(str) {
    return str.replace(/[A-Za-z]/g, char => {
        const start = char <= 'Z' ? 65 : 97;
        return String.fromCharCode((char.charCodeAt(0) - start + 13) % 26 + start);
    });
}

// The encoded message to decode
const encodedMessage = 'Pbatenghyngvbaf ba ohvyqvat n pbqr-rqvgvat ntrag!';

// Decode and print the message
const decodedMessage = rot13(encodedMessage);
console.log(decodedMessage);
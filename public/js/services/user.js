scionApp
    .factory('userService', ["$http", "$q", function ($http, $q) {
    return {
        // Load the user's data
        userPageData: function () {
            return $http.get('/api/userPageData').then(function (response) {
                console.log(response);
                return response.data;
            });
        },
        // Create SCIONLab AS
        generateSCIONLabAS: function (user) {
            // TODO(ercanucan): compose the URL in a cleaner fashion
            let url = '/api/as/generateAS?isVPN=' + (!user.isNotVPN) + '&scionLabASIP='
                + user.scionLabASIP;
            return $http.post(url).then(function (response) {
                console.log(response);
                return response.data;
            });
        },
        // Remove SCIONLab AS
        removeSCIONLabAS: function (user) {
            return $http.post('/api/as/removeAS').then(function (response) {
                console.log(response);
                return response.data;
            });
        }
    };
}]);

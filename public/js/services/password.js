scionApp
    .factory('passwordService', ["$http", "$q", function ($http, $q) {
    return {
        // set the password for a user
        setPassword: function (user) {
            // $http returns a promise, which has a then function, which also returns a promise
            return $http.post('/api/setPassword', user).then(function (response) {
                // The then function here is an opportunity to modify the response
                console.log(response);
                // The return value gets picked up by the then in the controller.
                return response.data;
            });
        }
    };
}]);
